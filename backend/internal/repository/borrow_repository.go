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
	CountByStatus(ctx context.Context) (map[constants.BorrowStatus]int64, error)
	ListOverdue(ctx context.Context, limit int) ([]model.BorrowRecord, error)
	TopEquipmentThisMonth(ctx context.Context, limit int) ([]BorrowTopStat, error)
	RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error
	// ExtendExpectedReturn 仅在借用仍为已通过、到期日未变化且尚未到期时延长归还日期，返回受影响行数。
	ExtendExpectedReturn(ctx context.Context, borrowID uint, oldDueDate, newDueDate, now time.Time) (int64, error)
	// MarkApprovedOverdue 将已通过且到期的借用批量标记为逾期，返回受影响行数。
	MarkApprovedOverdue(ctx context.Context, now time.Time) (int64, error)
	// ExpirePendingRenewalsForOverdue 将逾期借用下待审批的续借申请批量置为失效，返回受影响行数。
	ExpirePendingRenewalsForOverdue(ctx context.Context, now time.Time) (int64, error)
	// ExpirePendingRenewalsForBorrow 将指定借用下待审批的续借申请置为失效。
	ExpirePendingRenewalsForBorrow(ctx context.Context, borrowID uint) error
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

// RunInTx 在借用仓储所在数据库上执行事务。
func (r *borrowRepository) RunInTx(ctx context.Context, fn func(txCtx context.Context) error) error {
	if err := RunInTransaction(ctx, r.db, fn); err != nil {
		return fmt.Errorf("borrow transaction: %w", err)
	}
	return nil
}

func (r *borrowRepository) Create(ctx context.Context, record *model.BorrowRecord) error {
	if err := dbFromContext(ctx, r.db).Create(record).Error; err != nil {
		return fmt.Errorf("create borrow record: %w", err)
	}
	return nil
}

func (r *borrowRepository) Update(ctx context.Context, record *model.BorrowRecord) error {
	if err := dbFromContext(ctx, r.db).Save(record).Error; err != nil {
		return fmt.Errorf("update borrow record: %w", err)
	}
	return nil
}

func (r *borrowRepository) FindByID(ctx context.Context, id uint) (*model.BorrowRecord, error) {
	var record model.BorrowRecord
	err := dbFromContext(ctx, r.db).
		Preload("Equipment.Category").Preload("Borrower.Role").Preload("Approver.Role").
		Preload("Renewals", "status = ?", constants.RenewalStatusPending).
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
	query := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Preload("Equipment.Category").Preload("Borrower.Role").Preload("Approver.Role").
		Preload("Renewals", "status = ?", constants.RenewalStatusPending)
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
	if err := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Where("status = ?", constants.BorrowStatusPending).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count pending borrow records: %w", err)
	}
	return count, nil
}

func (r *borrowRepository) CountOverdue(ctx context.Context) (int64, error) {
	var count int64
	if err := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Where("status = ?", constants.BorrowStatusOverdue).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count overdue borrow records: %w", err)
	}
	return count, nil
}

func (r *borrowRepository) CountByStatus(ctx context.Context) (map[constants.BorrowStatus]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	rows := make([]statusCount, 0)
	if err := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Select("status, count(*) as count").
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count borrow records by status: %w", err)
	}
	result := make(map[constants.BorrowStatus]int64, len(rows))
	for _, row := range rows {
		result[constants.BorrowStatus(row.Status)] = row.Count
	}
	return result, nil
}

func (r *borrowRepository) ListOverdue(ctx context.Context, limit int) ([]model.BorrowRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	var list []model.BorrowRecord
	err := dbFromContext(ctx, r.db).
		Preload("Equipment.Category").Preload("Borrower.Role").
		Where("status = ?", constants.BorrowStatusOverdue).
		Order("expected_return_date ASC").
		Limit(limit).
		Find(&list).Error
	if err != nil {
		return nil, fmt.Errorf("list overdue borrow records: %w", err)
	}
	return list, nil
}

func (r *borrowRepository) ExtendExpectedReturn(ctx context.Context, borrowID uint, oldDueDate, newDueDate, now time.Time) (int64, error) {
	result := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Where("id = ? AND status = ? AND expected_return_date = ? AND expected_return_date >= ?",
			borrowID, constants.BorrowStatusApproved, oldDueDate, now).
		Update("expected_return_date", newDueDate)
	if result.Error != nil {
		return 0, fmt.Errorf("extend expected return date: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *borrowRepository) MarkApprovedOverdue(ctx context.Context, now time.Time) (int64, error) {
	result := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
		Where("status = ? AND expected_return_date < ?", constants.BorrowStatusApproved, now).
		Update("status", constants.BorrowStatusOverdue)
	if result.Error != nil {
		return 0, fmt.Errorf("mark approved borrows overdue: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *borrowRepository) ExpirePendingRenewalsForOverdue(ctx context.Context, now time.Time) (int64, error) {
	overdueSubQuery := r.db.Model(&model.BorrowRecord{}).
		Select("id").
		Where("status = ? AND expected_return_date < ?", constants.BorrowStatusApproved, now)
	result := dbFromContext(ctx, r.db).Model(&model.BorrowRenewal{}).
		Where("status = ?", constants.RenewalStatusPending).
		Where("borrow_id IN (?)", overdueSubQuery).
		Updates(map[string]any{
			"status":            constants.RenewalStatusExpired,
			"pending_borrow_id": nil,
		})
	if result.Error != nil {
		return 0, fmt.Errorf("expire pending renewals for overdue borrows: %w", result.Error)
	}
	return result.RowsAffected, nil
}

func (r *borrowRepository) ExpirePendingRenewalsForBorrow(ctx context.Context, borrowID uint) error {
	result := dbFromContext(ctx, r.db).Model(&model.BorrowRenewal{}).
		Where("borrow_id = ? AND status = ?", borrowID, constants.RenewalStatusPending).
		Updates(map[string]any{
			"status":            constants.RenewalStatusExpired,
			"pending_borrow_id": nil,
		})
	if result.Error != nil {
		return fmt.Errorf("expire pending renewals for borrow: %w", result.Error)
	}
	return nil
}

func (r *borrowRepository) TopEquipmentThisMonth(ctx context.Context, limit int) ([]BorrowTopStat, error) {
	if limit <= 0 {
		limit = 10
	}
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	rows := []BorrowTopStat{}
	err := dbFromContext(ctx, r.db).Model(&model.BorrowRecord{}).
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
