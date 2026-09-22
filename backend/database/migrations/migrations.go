package migrations

import (
	"fmt"
	"time"

	"github.com/labequipment/lab-equipment/internal/model"
	"gorm.io/gorm"
)

// AutoMigrate 自动迁移全部数据表。
func AutoMigrate(db *gorm.DB) error {
	models := []any{
		&model.Role{},
		&model.User{},
		&model.EquipmentCategory{},
		&model.Equipment{},
		&model.BorrowRecord{},
		&model.BorrowRenewal{},
		&model.MaintenanceRecord{},
		&model.Reservation{},
		&model.AuditLog{},
	}
	for _, m := range models {
		if err := db.AutoMigrate(m); err != nil {
			return fmt.Errorf("auto migrate %T: %w", m, err)
		}
	}
	if err := backfillOriginalExpectedReturn(db); err != nil {
		return fmt.Errorf("backfill original expected return: %w", err)
	}
	return nil
}

// backfillOriginalExpectedReturn 为迁移前已存在的借用记录补齐原始预计归还时间。
func backfillOriginalExpectedReturn(db *gorm.DB) error {
	var records []model.BorrowRecord
	if err := db.Where("original_expected_return IS NULL OR original_expected_return = ?", time.Time{}).
		Find(&records).Error; err != nil {
		return err
	}
	for i := range records {
		original := records[i].ExpectedReturnDate
		if original.IsZero() {
			original = time.Now()
		}
		if err := db.Model(&model.BorrowRecord{}).
			Where("id = ?", records[i].ID).
			Update("original_expected_return", original).Error; err != nil {
			return err
		}
	}
	return nil
}
