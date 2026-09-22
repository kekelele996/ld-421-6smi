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

// BorrowRepository 借用记录仓储接口。
type BorrowRepository interface {
	Create(ctx context.Context, record *model.BorrowRecord) error
	Update(ctx context.Context, record *model.BorrowRecord) error
	FindByID(ctx context.Context, id uint) (*model.BorrowRecord, error)
	List(ctx context.Context, filter BorrowFilter) ([]model.BorrowRecord, int64, error)
	CountPending(ctx context.Context) (int64, error)
	CountOverdue(ctx context.Context) (int64, error)
	// MarkOverdue 将预计归还日已过的已审批借用置为逾期，并同步关闭其待审批续借申请。
	MarkOverdue(ctx context.Context, now time.Time) (int64, error)
	TopEquipmentThisMonth(ctx context.Context, limit int) ([]BorrowTopStat, error)
}

// BorrowFilter 借用记录筛选条件。
type BorrowFilter struct {
	Status      constants.BorrowStatus
	EquipmentID uint
	BorrowerID  uint
	Pagination
}

// BorrowTopStat 本月借用次数统计结果。
type BorrowTopStat struct {
	EquipmentID uint
	Name        string
	Code        string
	Count       int64
}

type borrowRepository struct {
	db *gorm.DB
}

// NewBorrowRepository 构造借用记录仓储。
func NewBorrowRepository(db *gorm.DB) BorrowRepository {
	return &borrowRepository{db: db}
}

func (r *borrowRepository) Create(ctx context.Context, record *model.BorrowRecord) error {
	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		return fmt.Errorf("create borrow record: %w", err)
	}
	return nil
}

func (r *borrowRepository) Update(ctx context.Context, record *model.BorrowRecord) error {
	if err := r.db.WithContext(ctx).Save(record).Error; err != nil {
		return fmt.Errorf("update borrow record: %w", err)
	}
	return nil
}

func (r *borrowRepository) FindByID(ctx context.Context, id uint) (*model.BorrowRecord, error) {
	var record model.BorrowRecord
	err := r.db.WithContext(ctx).
		Preload("Equipment.Category").Preload("Borrower.Role").Preload("Approver.Role").
		Preload("Renewals", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		First(&record, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find borrow record by id: %w", err)
	}
	return &record, nil
}

func (r *borrowRepository) List(ctx context.Context, filter BorrowFilter) ([]model.BorrowRecord, int64, error) {
	filter.Normalize()
	query := r.db.WithContext(ctx).Model(&model.BorrowRecord{}).
		Preload("Equipment.Category").Preload("Borrower.Role").Preload("Approver.Role").
		Preload("Renewals", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") })
	if filter.Status.Valid() {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.EquipmentID > 0 {
		query = query.Where("equipment_id = ?", filter.EquipmentID)
	}
	if filter.BorrowerID > 0 {
		query = query.Where("borrower_id = ?", filter.BorrowerID)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count borrow records: %w", err)
	}
	var list []model.BorrowRecord
	if err := query.Order("id DESC").Offset(filter.Offset()).Limit(filter.PageSize).Find(&list).Error; err != nil {
		return nil, 0, fmt.Errorf("list borrow records: %w", err)
	}
	return list, total, nil
}

func (r *borrowRepository) CountPending(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BorrowRecord{}).
		Where("status = ?", constants.BorrowStatusPending).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count pending borrow records: %w", err)
	}
	return count, nil
}

func (r *borrowRepository) CountOverdue(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.BorrowRecord{}).
		Where("status = ?", constants.BorrowStatusOverdue).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count overdue borrow records: %w", err)
	}
	return count, nil
}

// MarkOverdue 扫描已审批且超过预计归还日的借用：置为逾期，并将其待审批续借申请驳回。
// 整个操作在单个事务内完成，重复调用幂等，返回本次受影响（进入逾期）的借用数量。
func (r *borrowRepository) MarkOverdue(ctx context.Context, now time.Time) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var borrowIDs []uint
		if err := tx.Model(&model.BorrowRecord{}).
			Where("status = ? AND expected_return_date < ?", constants.BorrowStatusApproved, now).
			Pluck("id", &borrowIDs).Error; err != nil {
			return fmt.Errorf("find due borrow records: %w", err)
		}
		affected = int64(len(borrowIDs))
		if len(borrowIDs) == 0 {
			return nil
		}
		if err := tx.Model(&model.BorrowRecord{}).
			Where("id IN ?", borrowIDs).
			Update("status", constants.BorrowStatusOverdue).Error; err != nil {
			return fmt.Errorf("mark borrow records overdue: %w", err)
		}
		// 逾期后待审批续借自动驳回，不改变借用本身的归还日期。
		if err := tx.Model(&model.BorrowRenewal{}).
			Where("borrow_id IN ? AND status = ?", borrowIDs, constants.RenewalStatusPending).
			Updates(map[string]any{
				"status":            constants.RenewalStatusRejected,
				"review_reason":     "借用已逾期，续借申请自动驳回",
				"pending_borrow_id": nil,
			}).Error; err != nil {
			return fmt.Errorf("auto reject pending renewals: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *borrowRepository) TopEquipmentThisMonth(ctx context.Context, limit int) ([]BorrowTopStat, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	rows := []BorrowTopStat{}
	err := r.db.WithContext(ctx).Model(&model.BorrowRecord{}).
		Select("borrow_records.equipment_id, equipment.name, equipment.code, count(*) as count").
		Joins("LEFT JOIN equipment ON equipment.id = borrow_records.equipment_id").
		Where("borrow_records.borrow_date >= ?", monthStart).
		Group("borrow_records.equipment_id, equipment.name, equipment.code").
		Order("count DESC").
		Limit(limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("top equipment this month: %w", err)
	}
	return rows, nil
}
