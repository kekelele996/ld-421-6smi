package repository

import (
	"context"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
)

func seedRenewalBorrow(t *testing.T, repo BorrowRepository, due time.Time) *model.BorrowRecord {
	t.Helper()
	record := &model.BorrowRecord{
		EquipmentID:            1,
		BorrowerID:             1,
		BorrowDate:             due.AddDate(0, 0, -7),
		ExpectedReturnDate:     due,
		OriginalExpectedReturn: due,
		Status:                 constants.BorrowStatusApproved,
	}
	if err := repo.Create(context.Background(), record); err != nil {
		t.Fatalf("create borrow: %v", err)
	}
	return record
}

func TestBorrowRenewalRepository_PendingUnique(t *testing.T) {
	db := newTestDB(t)
	repo := NewBorrowRenewalRepository(db)
	borrowRepo := NewBorrowRepository(db)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := seedRenewalBorrow(t, borrowRepo, due)

	first := &model.BorrowRenewal{
		BorrowID:         record.ID,
		ApplicantID:      1,
		ExtendDays:       2,
		CurrentDueDate:   due,
		RequestedDueDate: due.AddDate(0, 0, 2),
		Status:           constants.RenewalStatusPending,
		PendingBorrowID:  &record.ID,
	}
	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first renewal: %v", err)
	}
	second := *first
	second.ExtendDays = 3
	second.RequestedDueDate = due.AddDate(0, 0, 3)
	if err := repo.Create(ctx, &second); err == nil {
		t.Fatalf("second pending renewal for the same borrow must violate unique index")
	}
}

func TestBorrowRenewalRepository_MarkReviewedCAS(t *testing.T) {
	db := newTestDB(t)
	repo := NewBorrowRenewalRepository(db)
	borrowRepo := NewBorrowRepository(db)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := seedRenewalBorrow(t, borrowRepo, due)
	borrowID := record.ID
	renewal := &model.BorrowRenewal{
		BorrowID:         record.ID,
		ApplicantID:      1,
		ExtendDays:       2,
		CurrentDueDate:   due,
		RequestedDueDate: due.AddDate(0, 0, 2),
		Status:           constants.RenewalStatusPending,
		PendingBorrowID:  &borrowID,
	}
	if err := repo.Create(ctx, renewal); err != nil {
		t.Fatalf("create renewal: %v", err)
	}

	rows, err := repo.MarkReviewed(ctx, renewal.ID, constants.RenewalStatusApproved, 9, "", time.Now())
	if err != nil {
		t.Fatalf("mark reviewed: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 affected row, got %d", rows)
	}
	// 再次审批不应命中任何行：并发/重复批准只生效一次。
	rows, err = repo.MarkReviewed(ctx, renewal.ID, constants.RenewalStatusApproved, 9, "", time.Now())
	if err != nil {
		t.Fatalf("duplicate mark reviewed: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 affected row on duplicate review, got %d", rows)
	}
	got, err := repo.FindByID(ctx, renewal.ID)
	if err != nil {
		t.Fatalf("find renewal: %v", err)
	}
	if got.Status != constants.RenewalStatusApproved || got.PendingBorrowID != nil {
		t.Fatalf("unexpected reviewed renewal: %+v", got)
	}
}

func TestBorrowRepository_ExtendCASAndOverdueSweep(t *testing.T) {
	db := newTestDB(t)
	repo := NewBorrowRepository(db)
	ctx := context.Background()
	futureDue := time.Now().AddDate(0, 0, 5)
	pastDue := time.Now().AddDate(0, 0, -2)
	active := seedRenewalBorrow(t, repo, futureDue)
	overdue := seedRenewalBorrow(t, repo, pastDue)

	// CAS 条件不匹配（到期日已变化）时不更新。
	rows, err := repo.ExtendExpectedReturn(ctx, active.ID, futureDue.AddDate(0, 0, 1), futureDue.AddDate(0, 0, 4), time.Now())
	if err != nil {
		t.Fatalf("extend mismatch: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected 0 rows on stale due date, got %d", rows)
	}
	rows, err = repo.ExtendExpectedReturn(ctx, active.ID, futureDue, futureDue.AddDate(0, 0, 3), time.Now())
	if err != nil {
		t.Fatalf("extend: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected 1 row extended, got %d", rows)
	}

	marked, err := repo.MarkApprovedOverdue(ctx, time.Now())
	if err != nil {
		t.Fatalf("mark overdue: %v", err)
	}
	if marked != 1 {
		t.Fatalf("expected 1 overdue borrow, got %d", marked)
	}
	count, err := repo.CountOverdue(ctx)
	if err != nil {
		t.Fatalf("count overdue: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected overdue count 1, got %d", count)
	}
	got, _ := repo.FindByID(ctx, overdue.ID)
	if got.Status != constants.BorrowStatusOverdue {
		t.Fatalf("expected overdue status, got %s", got.Status)
	}
	extended, _ := repo.FindByID(ctx, active.ID)
	if extended.Status != constants.BorrowStatusApproved {
		t.Fatalf("extended borrow must stay approved, got %s", extended.Status)
	}
}
