package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
	"gorm.io/gorm"
)

func seedBorrowForRenewal(t *testing.T, db *gorm.DB, dueDate time.Time) uint {
	t.Helper()
	record := model.BorrowRecord{
		EquipmentID:        1,
		BorrowerID:         1,
		BorrowDate:         dueDate.AddDate(0, 0, -5),
		ExpectedReturnDate: dueDate,
		Status:             constants.BorrowStatusApproved,
	}
	if err := db.Create(&record).Error; err != nil {
		t.Fatalf("seed borrow: %v", err)
	}
	return record.ID
}

func TestRenewalRepository_CreateIfNoPending(t *testing.T) {
	db := newTestDB(t)
	borrowRepo := NewBorrowRepository(db)
	repo := NewRenewalRepository(db)
	ctx := context.Background()
	borrowID := seedBorrowForRenewal(t, db, time.Now().AddDate(0, 0, 5))

	first := &model.BorrowRenewal{BorrowID: borrowID, ExtendDays: 2, NewDueDate: time.Now().AddDate(0, 0, 7)}
	if err := repo.CreateIfNoPending(ctx, first); err != nil {
		t.Fatalf("first create: %v", err)
	}
	second := &model.BorrowRenewal{BorrowID: borrowID, ExtendDays: 3, NewDueDate: time.Now().AddDate(0, 0, 8)}
	if err := repo.CreateIfNoPending(ctx, second); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// 驳回后可再次创建。
	if err := repo.Reject(ctx, first.ID, 1, "x"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	third := &model.BorrowRenewal{BorrowID: borrowID, ExtendDays: 1, NewDueDate: time.Now().AddDate(0, 0, 6)}
	if err := repo.CreateIfNoPending(ctx, third); err != nil {
		t.Fatalf("create after reject: %v", err)
	}
	_ = borrowRepo
}

func TestRenewalRepository_ApproveConcurrentOnce(t *testing.T) {
	db := newTestDB(t)
	repo := NewRenewalRepository(db)
	borrowRepo := NewBorrowRepository(db)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	borrowID := seedBorrowForRenewal(t, db, due)

	renewal := &model.BorrowRenewal{BorrowID: borrowID, ExtendDays: 3, NewDueDate: due.AddDate(0, 0, 3)}
	if err := repo.CreateIfNoPending(ctx, renewal); err != nil {
		t.Fatalf("create: %v", err)
	}
	newDue := renewal.NewDueDate
	if err := repo.Approve(ctx, renewal.ID, 9, newDue); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// 第二次批准（并发场景）必须失败。
	if err := repo.Approve(ctx, renewal.ID, 9, newDue); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on second approve, got %v", err)
	}
	record, err := borrowRepo.FindByID(ctx, borrowID)
	if err != nil {
		t.Fatalf("find borrow: %v", err)
	}
	if !record.ExpectedReturnDate.Equal(newDue) {
		t.Fatalf("due not updated: %s", record.ExpectedReturnDate)
	}
	if record.OriginalDueDate == nil || !record.OriginalDueDate.Equal(due) {
		t.Fatalf("original due not kept: %v", record.OriginalDueDate)
	}
}

func TestBorrowRepository_MarkOverdue(t *testing.T) {
	db := newTestDB(t)
	borrowRepo := NewBorrowRepository(db)
	renewalRepo := NewRenewalRepository(db)
	ctx := context.Background()

	pastID := seedBorrowForRenewal(t, db, time.Now().AddDate(0, 0, -2))
	futureID := seedBorrowForRenewal(t, db, time.Now().AddDate(0, 0, 3))

	renewal := &model.BorrowRenewal{BorrowID: pastID, ExtendDays: 2, NewDueDate: time.Now()}
	if err := renewalRepo.CreateIfNoPending(ctx, renewal); err != nil {
		t.Fatalf("create renewal: %v", err)
	}

	affected, err := borrowRepo.MarkOverdue(ctx, time.Now())
	if err != nil {
		t.Fatalf("mark overdue: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected, got %d", affected)
	}
	past, _ := borrowRepo.FindByID(ctx, pastID)
	if past.Status != constants.BorrowStatusOverdue {
		t.Fatalf("past borrow should be overdue, got %s", past.Status)
	}
	future, _ := borrowRepo.FindByID(ctx, futureID)
	if future.Status != constants.BorrowStatusApproved {
		t.Fatalf("future borrow should stay approved, got %s", future.Status)
	}
	r, err := renewalRepo.FindByID(ctx, renewal.ID)
	if err != nil {
		t.Fatalf("find renewal: %v", err)
	}
	if r.Status != constants.RenewalStatusRejected {
		t.Fatalf("pending renewal should be rejected on overdue, got %s", r.Status)
	}

	// 幂等：再次扫描无新增。
	affected2, err := borrowRepo.MarkOverdue(ctx, time.Now())
	if err != nil || affected2 != 0 {
		t.Fatalf("idempotent mark expected 0, got %d err=%v", affected2, err)
	}
}
