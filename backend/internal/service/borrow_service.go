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

// BorrowService 借用记录业务服务。
type BorrowService struct {
	repo          repository.BorrowRepository
	equipmentRepo repository.EquipmentRepository
	audit         *AuditService
	logger        *slog.Logger
}

// NewBorrowService 构造借用服务。
func NewBorrowService(
	repo repository.BorrowRepository,
	equipmentRepo repository.EquipmentRepository,
	audit *AuditService,
	logger *slog.Logger,
) *BorrowService {
	return &BorrowService{repo: repo, equipmentRepo: equipmentRepo, audit: audit, logger: logger}
}

// Create 提交借用申请。
func (s *BorrowService) Create(ctx context.Context, record *model.BorrowRecord, actor Actor) (*model.BorrowRecord, error) {
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewBusinessError(40400, 404, "设备不存在")
		}
		return nil, fmt.Errorf("find equipment: %w", err)
	}
	if equipment.Status != constants.AssetStatusAvailable {
		return nil, apperrors.NewBusinessError(40900, 409, "设备当前不可借用")
	}
	if record.ExpectedReturnDate.Before(record.BorrowDate) {
		return nil, apperrors.NewBusinessError(40000, 400, "预计归还日期不能早于借用日期")
	}
	record.BorrowerID = actor.UserID
	record.Status = constants.BorrowStatusPending
	// 保留首次预计归还时间，续借批准只更新预计归还日期，不改原值。
	record.OriginalExpectedReturn = record.ExpectedReturnDate
	if err := s.repo.Create(ctx, record); err != nil {
		return nil, fmt.Errorf("create borrow record: %w", err)
	}
	equipment.Status = constants.AssetStatusInUse
	if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
		return nil, fmt.Errorf("mark equipment in use: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.submit", "borrow", record.ID, fmt.Sprintf("提交借用设备 %d", record.EquipmentID)); err != nil {
		return nil, err
	}
	return record, nil
}

// Approve 审批通过借用。
func (s *BorrowService) Approve(ctx context.Context, id uint, actor Actor) error {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if record.Status != constants.BorrowStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批记录可审批")
	}
	approverID := actor.UserID
	record.Status = constants.BorrowStatusApproved
	record.ApproverID = &approverID
	if err := s.repo.Update(ctx, record); err != nil {
		return fmt.Errorf("approve borrow record: %w", err)
	}
	if err := s.audit.Log(ctx, actor, "borrow.approve", "borrow", id, fmt.Sprintf("审批通过借用记录 %d", id)); err != nil {
		return err
	}
	return nil
}

// Reject 驳回借用申请。
func (s *BorrowService) Reject(ctx context.Context, id uint, actor Actor) error {
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if record.Status != constants.BorrowStatusPending {
		return apperrors.NewBusinessError(40900, 409, "仅待审批记录可驳回")
	}
	approverID := actor.UserID
	record.Status = constants.BorrowStatusRejected
	record.ApproverID = &approverID
	if err := s.repo.Update(ctx, record); err != nil {
		return fmt.Errorf("reject borrow record: %w", err)
	}
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err == nil {
		equipment.Status = constants.AssetStatusAvailable
		if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
			return fmt.Errorf("reset equipment available: %w", err)
		}
	}
	if err := s.audit.Log(ctx, actor, "borrow.reject", "borrow", id, fmt.Sprintf("驳回借用记录 %d", id)); err != nil {
		return err
	}
	return nil
}

// Return 确认归还。
func (s *BorrowService) Return(ctx context.Context, id uint, actualReturnDate time.Time, condition constants.ReturnCondition, actor Actor) error {
	s.syncOverdue(ctx)
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return s.mapNotFound(err)
	}
	if record.Status != constants.BorrowStatusApproved && record.Status != constants.BorrowStatusOverdue {
		return apperrors.NewBusinessError(40900, 409, "仅已审批或逾期的借用可归还")
	}
	if !condition.Valid() {
		return apperrors.NewBusinessError(40000, 400, "归还状况无效")
	}
	record.Status = constants.BorrowStatusReturned
	record.ActualReturnDate = &actualReturnDate
	record.ReturnCondition = &condition
	// 归还关闭续借入口：同事务内将待审批续借申请置为失效。
	if err := s.repo.RunInTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.ExpirePendingRenewalsForBorrow(txCtx, id); err != nil {
			return err
		}
		return s.repo.Update(txCtx, record)
	}); err != nil {
		return fmt.Errorf("return borrow record: %w", err)
	}
	equipment, err := s.equipmentRepo.FindByID(ctx, record.EquipmentID)
	if err == nil {
		if condition == constants.ReturnConditionLost {
			equipment.Status = constants.AssetStatusLost
		} else {
			equipment.Status = constants.AssetStatusAvailable
		}
		if err := s.equipmentRepo.Update(ctx, equipment); err != nil {
			return fmt.Errorf("mark equipment available: %w", err)
		}
	}
	if err := s.audit.Log(ctx, actor, "borrow.return", "borrow", id, fmt.Sprintf("确认归还借用记录 %d", id)); err != nil {
		return err
	}
	return nil
}

// SweepOverdue 将到期未归还的借用自动置为逾期，并失效其待审批续借申请。
func (s *BorrowService) SweepOverdue(ctx context.Context) error {
	if err := s.repo.RunInTx(ctx, func(txCtx context.Context) error {
		now := time.Now()
		// 先失效到期借用下待审批的续借申请，逾期不改变原归还日期。
		if _, err := s.repo.ExpirePendingRenewalsForOverdue(txCtx, now); err != nil {
			return err
		}
		if _, err := s.repo.MarkApprovedOverdue(txCtx, now); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return fmt.Errorf("sweep overdue borrows: %w", err)
	}
	return nil
}

// syncOverdue 读前惰性同步逾期状态，失败仅告警，不阻断查询。
func (s *BorrowService) syncOverdue(ctx context.Context) {
	if err := s.SweepOverdue(ctx); err != nil {
		s.logger.WarnContext(ctx, "sweep overdue borrows failed", "error", err)
	}
}

// Get 获取借用详情。
func (s *BorrowService) Get(ctx context.Context, id uint) (*model.BorrowRecord, error) {
	s.syncOverdue(ctx)
	record, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, s.mapNotFound(err)
	}
	return record, nil
}

// List 分页查询借用记录。
func (s *BorrowService) List(ctx context.Context, filter repository.BorrowFilter) ([]model.BorrowRecord, int64, error) {
	s.syncOverdue(ctx)
	list, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list borrow records: %w", err)
	}
	return list, total, nil
}

func (s *BorrowService) mapNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return apperrors.NewBusinessError(40400, 404, "借用记录不存在")
	}
	return fmt.Errorf("find borrow record: %w", err)
}
