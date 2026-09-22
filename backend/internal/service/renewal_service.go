package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	apperrors "github.com/labequipment/lab-equipment/internal/errors"
	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
)

// 续借允许延长的天数范围。
const (
	minRenewalDays = 1
	maxRenewalDays = 7
)

// RenewalService 续借申请业务服务。
type RenewalService struct {
	renewalRepo repository.RenewalRepository
	borrowRepo  repository.BorrowRepository
	audit       *AuditService
	logger      *slog.Logger
}

// NewRenewalService 构造续借服务。
func NewRenewalService(
	renewalRepo repository.RenewalRepository,
	borrowRepo repository.BorrowRepository,
	audit *AuditService,
	logger *slog.Logger,
) *RenewalService {
	return &RenewalService{renewalRepo: renewalRepo, borrowRepo: borrowRepo, audit: audit, logger: logger}
}

// Apply 借用人提交续借申请。
func (s *RenewalService) Apply(ctx context.Context, borrowID uint, extendDays int, actor Actor) (*model.BorrowRenewal, error) {
	if extendDays < minRenewalDays || extendDays > maxRenewalDays {
		return nil, apperrors.NewBusinessError(40000, 400, "延长天数须在 1 至 7 天之间")
	}
	// 申请前同步逾期，确保逾期借用不会再产生续借申请。
	if _, err := s.borrowRepo.MarkOverdue(ctx, todayStart(time.Now())); err != nil {
		return nil, fmt.Errorf("sync overdue before renewal apply: %w", err)
	}
	record, err := s.borrowRepo.FindByID(ctx, borrowID)
	if err != nil {
		return nil, mapBorrowNotFound(err)
	}
	if record.BorrowerID != actor.UserID {
		return nil, apperrors.NewBusinessError(40300, 403, "仅借用人本人可申请续借")
	}
	if record.Status != constants.BorrowStatusApproved {
		return nil, apperrors.NewBusinessError(40900, 409, "仅审批通过的借用可申请续借")
	}
	// 申请须在预计归还日之前提出（按自然日，到期当日不可申请）。
	if !todayStart(time.Now()).Before(record.ExpectedReturnDate) {
		return nil, apperrors.NewBusinessError(40900, 409, "已到预计归还日或已逾期，无法申请续借")
	}
	renewal := &model.BorrowRenewal{
		BorrowID:   borrowID,
		ExtendDays: extendDays,
		NewDueDate: record.ExpectedReturnDate.AddDate(0, 0, extendDays),
	}
	if err := s.renewalRepo.CreateIfNoPending(ctx, renewal); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return nil, apperrors.NewBusinessError(40900, 409, "该笔借用已存在待审批续借申请")
		}
		return nil, fmt.Errorf("create renewal: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.renewal.apply", "renewal", renewal.ID, fmt.Sprintf("借用 %d 申请续借 %d 天", borrowID, extendDays)); err != nil {
		return nil, err
	}
	return renewal, nil
}

// Approve 管理员批准续借：更新归还日期，保留原始到期时间。并发批准只生效一次。
func (s *RenewalService) Approve(ctx context.Context, id uint, actor Actor) error {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		return mapRenewalNotFound(err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批续借申请可批准")
	}
	if renewal.Borrow == nil {
		return apperrors.NewBusinessError(40400, 404, "续借关联的借用记录不存在")
	}
	if renewal.Borrow.Status != constants.BorrowStatusApproved {
		return apperrors.NewBusinessError(40900, 409, "借用已逾期或已关闭，续借申请不可批准")
	}
	if !todayStart(time.Now()).Before(renewal.Borrow.ExpectedReturnDate) {
		return apperrors.NewBusinessError(40900, 409, "已过预计归还日，续借申请不可批准")
	}
	if err := s.renewalRepo.Approve(ctx, id, actor.UserID, renewal.NewDueDate); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return apperrors.NewBusinessError(40900, 409, "续借申请已处理或借用状态已变更，批准未生效")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return mapRenewalNotFound(err)
		}
		return fmt.Errorf("approve renewal: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.renewal.approve", "renewal", id, fmt.Sprintf("批准续借申请 %d，新归还日 %s", id, renewal.NewDueDate.Format("2006-01-02"))); err != nil {
		return err
	}
	return nil
}

// Reject 管理员驳回续借申请，驳回不改变借用原记录。
func (s *RenewalService) Reject(ctx context.Context, id uint, reason string, actor Actor) error {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		return mapRenewalNotFound(err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批续借申请可驳回")
	}
	if err := s.renewalRepo.Reject(ctx, id, actor.UserID, reason); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return apperrors.NewBusinessError(40900, 409, "续借申请已被处理，驳回未生效")
		}
		if errors.Is(err, repository.ErrNotFound) {
			return mapRenewalNotFound(err)
		}
		return fmt.Errorf("reject renewal: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.renewal.reject", "renewal", id, fmt.Sprintf("驳回续借申请 %d", id)); err != nil {
		return err
	}
	return nil
}

// ListByBorrow 查询某笔借用的续借历史。
func (s *RenewalService) ListByBorrow(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error) {
	if _, err := s.borrowRepo.FindByID(ctx, borrowID); err != nil {
		return nil, mapBorrowNotFound(err)
	}
	list, err := s.renewalRepo.ListByBorrowID(ctx, borrowID)
	if err != nil {
		return nil, fmt.Errorf("list renewals: %w", err)
	}
	return list, nil
}

func mapBorrowNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NewBusinessError(40400, 404, "借用记录不存在")
	}
	return fmt.Errorf("find borrow record: %w", err)
}

func mapRenewalNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NewBusinessError(40400, 404, "续借申请不存在")
	}
	return fmt.Errorf("find renewal: %w", err)
}
