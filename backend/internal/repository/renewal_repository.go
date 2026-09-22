package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
	"gorm.io/gorm"
)

// RenewalRepository 续借申请仓储接口。
type RenewalRepository interface {
	// CreateIfNoPending 在不存在待审批续借时创建申请，否则返回 ErrConflict。
	CreateIfNoPending(ctx context.Context, renewal *model.BorrowRenewal) error
	FindByID(ctx context.Context, id uint) (*model.BorrowRenewal, error)
	ListByBorrowID(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error)
	CountPending(ctx context.Context) (int64, error)
	// DismissPendingByBorrow 关闭某笔借用未处理的续借申请（归还时调用）。
	DismissPendingByBorrow(ctx context.Context, borrowID uint, reason string) error
	// Approve 原子地批准续借：仅当申请仍待审批且借用仍为已审批时生效，
	// 更新归还日期并保留原始到期时间；并发调用只有一次成功，其余返回 ErrConflict。
	Approve(ctx context.Context, id uint, reviewerID uint, newDueDate time.Time) error
	// Reject 原子地驳回复借，返回 ErrConflict 表示申请已不是待审批状态。
	Reject(ctx context.Context, id uint, reviewerID uint, reason string) error
}

type renewalRepository struct {
	db *gorm.DB
}

// NewRenewalRepository 构造续申请仓储。
func NewRenewalRepository(db *gorm.DB) RenewalRepository {
	return &renewalRepository{db: db}
}

func (r *renewalRepository) CreateIfNoPending(ctx context.Context, renewal *model.BorrowRenewal) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&model.BorrowRenewal{}).
			Where("borrow_id = ? AND status = ?", renewal.BorrowID, constants.RenewalStatusPending).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count pending renewals: %w", err)
		}
		if count > 0 {
			return ErrConflict
		}
		borrowID := renewal.BorrowID
		renewal.Status = constants.RenewalStatusPending
		renewal.PendingBorrowID = &borrowID
		if err := tx.Create(renewal).Error; err != nil {
			if isDuplicateKeyErr(err) {
				return ErrConflict
			}
			return fmt.Errorf("create renewal: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *renewalRepository) FindByID(ctx context.Context, id uint) (*model.BorrowRenewal, error) {
	var renewal model.BorrowRenewal
	err := r.db.WithContext(ctx).
		Preload("Borrow.Equipment.Category").Preload("Borrow.Borrower.Role").Preload("Reviewer").
		First(&renewal, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find renewal by id: %w", err)
	}
	return &renewal, nil
}

func (r *renewalRepository) ListByBorrowID(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error) {
	var list []model.BorrowRenewal
	if err := r.db.WithContext(ctx).
		Preload("Reviewer").
		Where("borrow_id = ?", borrowID).
		Order("id ASC").
		Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list renewals by borrow id: %w", err)
	}
	return list, nil
}

func (r *renewalRepository) CountPending(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BorrowRenewal{}).
		Where("status = ?", constants.RenewalStatusPending).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count pending renewals: %w", err)
	}
	return count, nil
}

func (r *renewalRepository) DismissPendingByBorrow(ctx context.Context, borrowID uint, reason string) error {
	if err := r.db.WithContext(ctx).Model(&model.BorrowRenewal{}).
		Where("borrow_id = ? AND status = ?", borrowID, constants.RenewalStatusPending).
		Updates(map[string]any{
			"status":            constants.RenewalStatusRejected,
			"review_reason":     reason,
			"pending_borrow_id": nil,
		}).Error; err != nil {
		return fmt.Errorf("dismiss pending renewals: %w", err)
	}
	return nil
}

func (r *renewalRepository) Approve(ctx context.Context, id uint, reviewerID uint, newDueDate time.Time) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var renewal model.BorrowRenewal
		if err := tx.First(&renewal, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("find renewal: %w", err)
		}
		if renewal.Status != constants.RenewalStatusPending {
			return ErrConflict
		}
		// 条件更新保证并发批准只生效一次。
		result := tx.Model(&model.BorrowRenewal{}).
			Where("id = ? AND status = ?", id, constants.RenewalStatusPending).
			Updates(map[string]any{
				"status":            constants.RenewalStatusApproved,
				"reviewer_id":       reviewerID,
				"new_due_date":      newDueDate,
				"pending_borrow_id": nil,
			})
		if result.Error != nil {
			return fmt.Errorf("approve renewal: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrConflict
		}
		// 仅当借用仍为已审批时延长归还日期，原始到期时间只在首次续借时保留。
		result = tx.Model(&model.BorrowRecord{}).
			Where("id = ? AND status = ?", renewal.BorrowID, constants.BorrowStatusApproved).
			Updates(map[string]any{
				"expected_return_date": newDueDate,
				"original_due_date":    gorm.Expr("COALESCE(original_due_date, ?)", renewal.NewDueDate.AddDate(0, 0, -renewal.ExtendDays)),
			})
		if result.Error != nil {
			return fmt.Errorf("extend borrow due date: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrConflict
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *renewalRepository) Reject(ctx context.Context, id uint, reviewerID uint, reason string) error {
	result := r.db.WithContext(ctx).Model(&model.BorrowRenewal{}).
		Where("id = ? AND status = ?", id, constants.RenewalStatusPending).
		Updates(map[string]any{
			"status":            constants.RenewalStatusRejected,
			"reviewer_id":       reviewerID,
			"review_reason":     reason,
			"pending_borrow_id": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("reject renewal: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// 区分“申请不存在”与“已处理”。
		var count int64
		if err := r.db.WithContext(ctx).Model(&model.BorrowRenewal{}).Where("id = ?", id).Count(&count).Error; err != nil {
			return fmt.Errorf("check renewal exists: %w", err)
		}
		if count == 0 {
			return ErrNotFound
		}
		return ErrConflict
	}
	return nil
}

func isDuplicateKeyErr(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate entry") || strings.Contains(message, "unique constraint failed")
}
