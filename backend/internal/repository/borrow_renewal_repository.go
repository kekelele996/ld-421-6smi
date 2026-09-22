package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
	"gorm.io/gorm"
)

// BorrowRenewalRepository 续借申请仓储接口。
type BorrowRenewalRepository interface {
	Create(ctx context.Context, renewal *model.BorrowRenewal) error
	Update(ctx context.Context, renewal *model.BorrowRenewal) error
	FindByID(ctx context.Context, id uint) (*model.BorrowRenewal, error)
	FindPendingByBorrow(ctx context.Context, borrowID uint) (*model.BorrowRenewal, error)
	ListByBorrow(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error)
	List(ctx context.Context, filter RenewalFilter) ([]model.BorrowRenewal, int64, error)
	// CountPending 统计待审批的续借申请数量。
	CountPending(ctx context.Context) (int64, error)
	// MarkReviewed 仅在申请仍为待审批时更新审批结果，返回受影响行数。
	MarkReviewed(ctx context.Context, id uint, status constants.RenewalStatus, reviewerID uint, comment string, reviewedAt time.Time) (int64, error)
}

// RenewalFilter 续借申请筛选条件。
type RenewalFilter struct {
	Status   constants.RenewalStatus
	BorrowID uint
	Pagination
}

type borrowRenewalRepository struct {
	db *gorm.DB
}

// NewBorrowRenewalRepository 构造续借申请仓储。
func NewBorrowRenewalRepository(db *gorm.DB) BorrowRenewalRepository {
	return &borrowRenewalRepository{db: db}
}

func (r *borrowRenewalRepository) Create(ctx context.Context, renewal *model.BorrowRenewal) error {
	if err := dbFromContext(ctx, r.db).Create(renewal).Error; err != nil {
		return fmt.Errorf("create borrow renewal: %w", err)
	}
	return nil
}

func (r *borrowRenewalRepository) Update(ctx context.Context, renewal *model.BorrowRenewal) error {
	if err := dbFromContext(ctx, r.db).Save(renewal).Error; err != nil {
		return fmt.Errorf("update borrow renewal: %w", err)
	}
	return nil
}

func (r *borrowRenewalRepository) FindByID(ctx context.Context, id uint) (*model.BorrowRenewal, error) {
	var renewal model.BorrowRenewal
	err := dbFromContext(ctx, r.db).
		Preload("Borrow").Preload("Applicant.Role").Preload("Reviewer.Role").
		First(&renewal, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find borrow renewal by id: %w", err)
	}
	return &renewal, nil
}

func (r *borrowRenewalRepository) FindPendingByBorrow(ctx context.Context, borrowID uint) (*model.BorrowRenewal, error) {
	var renewal model.BorrowRenewal
	err := dbFromContext(ctx, r.db).
		Where("borrow_id = ? AND status = ?", borrowID, constants.RenewalStatusPending).
		First(&renewal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find pending renewal by borrow: %w", err)
	}
	return &renewal, nil
}

func (r *borrowRenewalRepository) ListByBorrow(ctx context.Context, borrowID uint) ([]model.BorrowRenewal, error) {
	var list []model.BorrowRenewal
	err := dbFromContext(ctx, r.db).
		Preload("Applicant.Role").Preload("Reviewer.Role").
		Where("borrow_id = ?", borrowID).
		Order("id DESC").
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list renewals by borrow: %w", err)
	}
	return list, nil
}

func (r *borrowRenewalRepository) List(ctx context.Context, filter RenewalFilter) ([]model.BorrowRenewal, int64, error) {
	filter.Normalize()
	query := dbFromContext(ctx, r.db).Model(&model.BorrowRenewal{}).
		Preload("Borrow.Equipment.Category").Preload("Applicant.Role").Preload("Reviewer.Role")
	if filter.Status.Valid() {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.BorrowID > 0 {
		query = query.Where("borrow_id = ?", filter.BorrowID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count borrow renewals: %w", err)
	}
	var list []model.BorrowRenewal
	if err := query.Order("id DESC").Offset(filter.Offset()).Limit(filter.PageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list borrow renewals: %w", err)
	}
	return list, total, nil
}

func (r *borrowRenewalRepository) CountPending(ctx context.Context) (int64, error) {
	var count int64
	if err := dbFromContext(ctx, r.db).Model(&model.BorrowRenewal{}).
		Where("status = ?", constants.RenewalStatusPending).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count pending renewals: %w", err)
	}
	return count, nil
}

func (r *borrowRenewalRepository) MarkReviewed(ctx context.Context, id uint, status constants.RenewalStatus, reviewerID uint, comment string, reviewedAt time.Time) (int64, error) {
	result := dbFromContext(ctx, r.db).Model(&model.BorrowRenewal{}).
		Where("id = ? AND status = ?", id, constants.RenewalStatusPending).
		Updates(map[string]any{
			"status":            status,
			"reviewer_id":       reviewerID,
			"review_comment":    comment,
			"reviewed_at":       reviewedAt,
			"pending_borrow_id": nil,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("mark renewal reviewed: %w", result.Error)
	}
	return result.RowsAffected, nil
}
