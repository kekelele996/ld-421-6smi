package service

import (
	"context"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
)

func createApprovedBorrow(t *testing.T, env *testEnv, expectedReturn time.Time) *model.BorrowRecord {
	t.Helper()
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "LabManager"}
	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         expectedReturn.AddDate(0, 0, -7),
		ExpectedReturnDate: expectedReturn,
	}, actor)
	if err != nil {
		t.Fatalf("create borrow: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, actor); err != nil {
		t.Fatalf("approve borrow: %v", err)
	}
	got, err := env.borrowRepo.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("reload borrow: %v", err)
	}
	return got
}

func TestRenewalService_ApplyApprove(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)

	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, "实验延期", actor)
	if err != nil {
		t.Fatalf("apply renewal: %v", err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		t.Fatalf("expected pending renewal, got %s", renewal.Status)
	}
	if !renewal.RequestedDueDate.Equal(due.AddDate(0, 0, 3)) {
		t.Fatalf("unexpected requested due date: %s", renewal.RequestedDueDate)
	}

	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); err != nil {
		t.Fatalf("approve renewal: %v", err)
	}
	got, err := env.borrowRepo.FindByID(ctx, record.ID)
	if err != nil {
		t.Fatalf("reload borrow: %v", err)
	}
	if !got.ExpectedReturnDate.Equal(due.AddDate(0, 0, 3)) {
		t.Fatalf("expected due extended to %s, got %s", due.AddDate(0, 0, 3), got.ExpectedReturnDate)
	}
	if !got.OriginalExpectedReturn.Equal(due) {
		t.Fatalf("original due must be preserved: want %s got %s", due, got.OriginalExpectedReturn)
	}
	updated, err := env.renewalRepo.FindByID(ctx, renewal.ID)
	if err != nil {
		t.Fatalf("reload renewal: %v", err)
	}
	if updated.Status != constants.RenewalStatusApproved || updated.ReviewerID == nil {
		t.Fatalf("renewal not approved: %+v", updated)
	}
}

func TestRenewalService_RejectKeepsRecord(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	renewal, err := env.renewalService.Apply(ctx, record.ID, 2, "", applicant)
	if err != nil {
		t.Fatalf("apply renewal: %v", err)
	}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Reject(ctx, renewal.ID, "不同意", approver); err != nil {
		t.Fatalf("reject renewal: %v", err)
	}
	got, _ := env.borrowRepo.FindByID(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(due) {
		t.Fatalf("reject must not change due date: %s", got.ExpectedReturnDate)
	}
	if got.Status != constants.BorrowStatusApproved {
		t.Fatalf("reject must not change borrow status: %s", got.Status)
	}
	// 驳回后可再次申请。
	if _, err := env.renewalService.Apply(ctx, record.ID, 1, "", applicant); err != nil {
		t.Fatalf("re-apply after reject should succeed: %v", err)
	}
}

func TestRenewalService_DuplicatePendingApply(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.renewalService.Apply(ctx, record.ID, 2, "", applicant); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if _, err := env.renewalService.Apply(ctx, record.ID, 3, "", applicant); err == nil {
		t.Fatalf("second pending apply must fail")
	}
}

func TestRenewalService_InvalidExtendDays(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	for _, days := range []int{0, -1, 8, 30} {
		if _, err := env.renewalService.Apply(ctx, record.ID, days, "", applicant); err == nil {
			t.Fatalf("extendDays=%d must fail", days)
		}
	}
}

func TestRenewalService_ApplyOnDueDateRejected(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	today := truncateToDay(time.Now())
	record := createApprovedBorrow(t, env, today)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.renewalService.Apply(ctx, record.ID, 2, "", applicant); err == nil {
		t.Fatalf("apply on the due date must fail")
	}
}

func TestRenewalService_ApproveOnlyOnce(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, "", applicant)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// 重复批准不得再次延长日期。
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); err == nil {
		t.Fatalf("concurrent/duplicate approve must fail")
	}
	got, _ := env.borrowRepo.FindByID(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(due.AddDate(0, 0, 3)) {
		t.Fatalf("due must be extended exactly once: %s", got.ExpectedReturnDate)
	}
}

func TestRenewalService_OverdueLifecycle(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	pastDue := time.Now().AddDate(0, 0, -2)
	record := createApprovedBorrow(t, env, pastDue)

	// 直接插入一条待审批续借（绕过服务层的归还日前校验）。
	pending := &model.BorrowRenewal{
		BorrowID:         record.ID,
		ApplicantID:      env.ownerID,
		ExtendDays:       3,
		CurrentDueDate:   pastDue,
		RequestedDueDate: pastDue.AddDate(0, 0, 3),
		Status:           constants.RenewalStatusPending,
		PendingBorrowID:  &record.ID,
	}
	if err := env.renewalRepo.Create(ctx, pending); err != nil {
		t.Fatalf("seed pending renewal: %v", err)
	}

	if err := env.borrowService.SweepOverdue(ctx); err != nil {
		t.Fatalf("sweep overdue: %v", err)
	}
	got, _ := env.borrowRepo.FindByID(ctx, record.ID)
	if got.Status != constants.BorrowStatusOverdue {
		t.Fatalf("expected overdue, got %s", got.Status)
	}
	// 逾期不改变原归还日期。
	if !got.ExpectedReturnDate.Equal(pastDue) {
		t.Fatalf("overdue must not change due date: %s", got.ExpectedReturnDate)
	}
	updated, _ := env.renewalRepo.FindByID(ctx, pending.ID)
	if updated.Status != constants.RenewalStatusExpired {
		t.Fatalf("pending renewal must expire with borrow, got %s", updated.Status)
	}

	// 逾期列表可见，且逾期后不得再申请续借。
	list, total, err := env.borrowService.List(ctx, repository.BorrowFilter{
		Status:     constants.BorrowStatusOverdue,
		Pagination: repository.Pagination{Page: 1, PageSize: 10},
	})
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("overdue list mismatch: total=%d len=%d err=%v", total, len(list), err)
	}
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	if _, err := env.renewalService.Apply(ctx, record.ID, 2, "", applicant); err == nil {
		t.Fatalf("overdue borrow must not allow renewal apply")
	}

	// 逾期后仍可归还，归还后续借入口关闭。
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionGood, approver); err != nil {
		t.Fatalf("return overdue: %v", err)
	}
	returned, _ := env.borrowRepo.FindByID(ctx, record.ID)
	if returned.Status != constants.BorrowStatusReturned {
		t.Fatalf("expected returned, got %s", returned.Status)
	}
}

func TestBorrowService_ReturnExpiresPendingRenewal(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	due := time.Now().AddDate(0, 0, 5)
	record := createApprovedBorrow(t, env, due)
	applicant := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, "", applicant)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionGood, approver); err != nil {
		t.Fatalf("return: %v", err)
	}
	updated, _ := env.renewalRepo.FindByID(ctx, renewal.ID)
	if updated.Status != constants.RenewalStatusExpired {
		t.Fatalf("pending renewal must expire on return, got %s", updated.Status)
	}
}
