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

// 续借可延长的天数范围。
const (
	MinRenewalExtendDays = 1
	MaxRenewalExtendDays = 7
)

// BorrowRenewalService 续借申请业务服务。
type BorrowRenewalService struct {
	renewalRepo repository.BorrowRenewalRepository
	borrowRepo  repository.BorrowRepository
	audit       *AuditService
	logger      *slog.Logger
}

// NewBorrowRenewalService 构造续借服务。
func NewBorrowRenewalService(
	renewalRepo repository.BorrowRenewalRepository,
	borrowRepo repository.BorrowRepository,
	audit *AuditService,
	logger *slog.Logger,
) *BorrowRenewalService {
	return &BorrowRenewalService{renewalRepo: renewalRepo, borrowRepo: borrowRepo, audit: audit, logger: logger}
}

// Apply 借用人提交续借申请。
func (s *BorrowRenewalService) Apply(ctx context.Context, borrowID uint, extendDays int, reason string, actor Actor) (*model.BorrowRenewal, error) {
	if extendDays < MinRenewalExtendDays || extendDays > MaxRenewalExtendDays {
		return nil, apperrors.NewBusinessError(40000, 400, fmt.Sprintf("延长期限须为 %d 至 %d 天", MinRenewalExtendDays, MaxRenewalExtendDays))
	}
	record, err := s.borrowRepo.FindByID(ctx, borrowID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewBusinessError(40400, 404, "借用记录不存在")
		}
		return nil, fmt.Errorf("find borrow record: %w", err)
	}
	if record.BorrowerID != actor.UserID {
		return nil, apperrors.NewBusinessError(40300, 403, "仅借用人本人可申请续借")
	}
	switch {
	case record.Status == constants.BorrowStatusReturned:
		return nil, apperrors.NewBusinessError(40900, 409, "借用已归还，不能续借")
	case record.Status == constants.BorrowStatusRejected:
		return nil, apperrors.NewBusinessError(40900, 409, "借用申请已被驳回，不能续借")
	case record.Status == constants.BorrowStatusPending:
		return nil, apperrors.NewBusinessError(40900, 409, "借用尚未审批通过，不能续借")
	case record.Status == constants.BorrowStatusOverdue:
		return nil, apperrors.NewBusinessError(40900, 409, "借用已逾期，不能续借")
	case record.Status != constants.BorrowStatusApproved:
		return nil, apperrors.NewBusinessError(40900, 409, "当前状态不能申请续借")
	}
	// 申请须在预计归还日之前提出（按自然日比较）。
	today := truncateToDay(time.Now())
	dueDay := truncateToDay(record.ExpectedReturnDate)
	if !today.Before(dueDay) {
		return nil, apperrors.NewBusinessError(40900, 409, "已到预计归还日，不能申请续借")
	}
	if _, err := s.renewalRepo.FindPendingByBorrow(ctx, borrowID); err == nil {
		return nil, apperrors.NewBusinessError(40900, 409, "该笔借用已有待审批的续借申请")
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check pending renewal: %w", err)
	}

	currentDue := record.ExpectedReturnDate
	newDue := currentDue.AddDate(0, 0, extendDays)
	renewal := &model.BorrowRenewal{
		BorrowID:         borrowID,
		ApplicantID:      actor.UserID,
		ExtendDays:       extendDays,
		CurrentDueDate:   currentDue,
		RequestedDueDate: newDue,
		Reason:           reason,
		Status:           constants.RenewalStatusPending,
		PendingBorrowID:  &borrowID,
	}
	// 待审批唯一性：DB 唯一索引兜底并发插入。
	if err := s.renewalRepo.Create(ctx, renewal); err != nil {
		if isDuplicateKeyErr(err) {
			return nil, apperrors.NewBusinessError(40900, 409, "该笔借用已有待审批的续借申请")
		}
		return nil, fmt.Errorf("create borrow renewal: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.renew.apply", "borrow_renewal", renewal.ID,
		fmt.Sprintf("申请续借借用记录 %d，延长 %d 天", borrowID, extendDays)); err != nil {
		return nil, err
	}
	return renewal, nil
}

// Approve 管理员批准续借：更新预计归还日期，保留原到期时间；并发批准只生效一次。
func (s *BorrowRenewalService) Approve(ctx context.Context, id uint, actor Actor) error {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		return mapRenewalNotFound(err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批续借申请可审批")
	}
	record, err := s.borrowRepo.FindByID(ctx, renewal.BorrowID)
	if err != nil {
		return mapRenewalNotFound(err)
	}
	if record.Status != constants.BorrowStatusApproved {
		return apperrors.NewBusinessError(40900, 409, "借用当前状态不允许批准续借")
	}
	now := time.Now()
	reviewerID := actor.UserID
	txErr := s.borrowRepo.RunInTx(ctx, func(txCtx context.Context) error {
		// CAS 1：占用续借申请，仅一条并发请求可成功。
		rows, err := s.renewalRepo.MarkReviewed(txCtx, id, constants.RenewalStatusApproved, reviewerID, "", now)
		if err != nil {
			return err
		}
		if rows == 0 {
			return apperrors.NewBusinessError(40900, 409, "续借申请已被处理，请勿重复审批")
		}
		// CAS 2：仅在借用仍已通过、到期日未被改动且尚未到期时延长，原到期时间保留于 OriginalExpectedReturn。
		rows, err = s.borrowRepo.ExtendExpectedReturn(txCtx, renewal.BorrowID, renewal.CurrentDueDate, renewal.RequestedDueDate, now)
		if err != nil {
			return err
		}
		if rows == 0 {
			return apperrors.NewBusinessError(40900, 409, "借用已到期或归还日期已变化，续借未生效")
		}
		return nil
	})
	if txErr != nil {
		return txErr
	}
	if err := s.audit.Log(ctx, actor, "borrow.renew.approve", "borrow_renewal", id,
		fmt.Sprintf("批准续借申请 %d，归还日期延至 %s", id, renewal.RequestedDueDate.Format("2006-01-02"))); err != nil {
		return err
	}
	return nil
}

// Reject 管理员驳回续借申请，驳回不改变原借用记录。
func (s *BorrowRenewalService) Reject(ctx context.Context, id uint, comment string, actor Actor) error {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		return mapRenewalNotFound(err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批续借申请可驳回")
	}
	rows, err := s.renewalRepo.MarkReviewed(ctx, id, constants.RenewalStatusRejected, actor.UserID, comment, time.Now())
	if err != nil {
		return fmt.Errorf("reject renewal: %w", err)
	}
	if rows == 0 {
		return apperrors.NewBusinessError(40900, 409, "续借申请已被处理，请勿重复审批")
	}
	if err := s.audit.Log(ctx, actor, "borrow.renew.reject", "borrow_renewal", id, fmt.Sprintf("驳回续借申请 %d", id)); err != nil {
		return err
	}
	return nil
}

// Get 获取续借申请详情。
func (s *BorrowRenewalService) Get(ctx context.Context, id uint) (*model.BorrowRenewal, error) {
	renewal, err := s.renewalRepo.FindByID(ctx, id)
	if err != nil {
		return nil, mapRenewalNotFound(err)
	}
	return renewal, nil
}

// ListByBorrow 查询单笔借用的续借历史。
func (s *BorrowRenewalService) ListByBorrow(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error) {
	list, err := s.renewalRepo.ListByBorrow(ctx, borrowID)
	if err != nil {
		return nil, fmt.Errorf("list renewals by borrow: %w", err)
	}
	return list, nil
}

// List 分页查询续借申请。
func (s *BorrowRenewalService) List(ctx context.Context, filter repository.RenewalFilter) ([]model.BorrowRenewal, int64, error) {
	list, total, err := s.renewalRepo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list borrow renewals: %w", err)
	}
	return list, total, nil
}

func truncateToDay(t time.Time) time.Time {
	local := t.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func mapRenewalNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NewBusinessError(40400, 404, "续借申请不存在")
	}
	return fmt.Errorf("find borrow renewal: %w", err)
}
