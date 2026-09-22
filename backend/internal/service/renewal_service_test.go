package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
	apperrors "github.com/labequipment/lab-equipment/internal/errors"
	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
)

func isBusinessStatus(err error, httpStatus int) bool {
	var be *apperrors.BusinessError
	return errors.As(err, &be) && be.HTTPStatus == httpStatus
}

// createApprovedBorrow 创建一条审批通过、预计 future 天后归还的借用。
func createApprovedBorrow(t *testing.T, env *testEnv, actor Actor, future int) *model.BorrowRecord {
	t.Helper()
	ctx := context.Background()
	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now(),
		ExpectedReturnDate: time.Now().AddDate(0, 0, future),
	}, actor)
	if err != nil {
		t.Fatalf("create borrow: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, actor); err != nil {
		t.Fatalf("approve borrow: %v", err)
	}
	got, err := env.borrowService.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("get borrow: %v", err)
	}
	return got
}

func TestRenewalService_ApplyApproveKeepsOriginalDue(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	record := createApprovedBorrow(t, env, actor, 7)
	originalDue := record.ExpectedReturnDate

	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, actor)
	if err != nil {
		t.Fatalf("apply renewal: %v", err)
	}
	if renewal.Status != constants.RenewalStatusPending {
		t.Fatalf("expected pending renewal, got %s", renewal.Status)
	}

	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); err != nil {
		t.Fatalf("approve renewal: %v", err)
	}
	got, err := env.borrowService.Get(ctx, record.ID)
	if err != nil {
		t.Fatalf("get borrow: %v", err)
	}
	if !got.ExpectedReturnDate.Equal(originalDue.AddDate(0, 0, 3)) {
		t.Fatalf("due date not extended: got %s want %s", got.ExpectedReturnDate, originalDue.AddDate(0, 0, 3))
	}
	if got.OriginalDueDate == nil || !got.OriginalDueDate.Equal(originalDue) {
		t.Fatalf("original due date not preserved: got %v want %s", got.OriginalDueDate, originalDue)
	}

	// 第二次续借在新到期日基础上再次延长，原始到期时间保持不变。
	second, err := env.renewalService.Apply(ctx, record.ID, 2, actor)
	if err != nil {
		t.Fatalf("apply second renewal: %v", err)
	}
	if err := env.renewalService.Approve(ctx, second.ID, approver); err != nil {
		t.Fatalf("approve second renewal: %v", err)
	}
	got, _ = env.borrowService.Get(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(originalDue.AddDate(0, 0, 5)) {
		t.Fatalf("second extension wrong: got %s", got.ExpectedReturnDate)
	}
	if got.OriginalDueDate == nil || !got.OriginalDueDate.Equal(originalDue) {
		t.Fatalf("original due changed after second renewal: %v", got.OriginalDueDate)
	}
}

func TestRenewalService_OnlyOnePendingPerBorrow(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	record := createApprovedBorrow(t, env, actor, 7)

	if _, err := env.renewalService.Apply(ctx, record.ID, 2, actor); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	_, err := env.renewalService.Apply(ctx, record.ID, 5, actor)
	if !isBusinessStatus(err, 409) {
		t.Fatalf("expected 409 for duplicate pending renewal, got %v", err)
	}
}

func TestRenewalService_InvalidDays(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	record := createApprovedBorrow(t, env, actor, 7)

	for _, days := range []int{0, -1, 8, 100} {
		if _, err := env.renewalService.Apply(ctx, record.ID, days, actor); !isBusinessStatus(err, 400) {
			t.Fatalf("days=%d expected 400, got %v", days, err)
		}
	}
}

func TestRenewalService_ApproveOnlyOnce(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	record := createApprovedBorrow(t, env, actor, 7)
	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, actor)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// 并发/重复批准只生效一次。
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); !isBusinessStatus(err, 409) {
		t.Fatalf("second approve expected 409, got %v", err)
	}
	got, _ := env.borrowService.Get(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(record.ExpectedReturnDate.AddDate(0, 0, 3)) {
		t.Fatalf("due date extended more than once: %s", got.ExpectedReturnDate)
	}
}

func TestRenewalService_RejectKeepsRecord(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	record := createApprovedBorrow(t, env, actor, 7)
	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, actor)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	if err := env.renewalService.Reject(ctx, renewal.ID, "不同意", approver); err != nil {
		t.Fatalf("reject: %v", err)
	}
	got, _ := env.borrowService.Get(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(record.ExpectedReturnDate) {
		t.Fatalf("reject changed due date: %s", got.ExpectedReturnDate)
	}
	if got.OriginalDueDate != nil {
		t.Fatalf("reject should not set original due date")
	}
	if got.Status != constants.BorrowStatusApproved {
		t.Fatalf("status changed after reject: %s", got.Status)
	}
	// 驳回后可以重新申请。
	if _, err := env.renewalService.Apply(ctx, record.ID, 1, actor); err != nil {
		t.Fatalf("re-apply after reject: %v", err)
	}
}

func TestRenewalService_OverdueCannotApplyAndAutoMarked(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}

	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now().AddDate(0, 0, -10),
		ExpectedReturnDate: time.Now().AddDate(0, 0, -1),
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, approver); err != nil {
		t.Fatalf("approve: %v", err)
	}

	// 列表查询触发自动逾期。
	list, total, err := env.borrowService.List(ctx, repository.BorrowFilter{BorrowerID: actor.UserID, Pagination: repository.Pagination{Page: 1, PageSize: 10}})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || list[0].Status != constants.BorrowStatusOverdue {
		t.Fatalf("expected auto overdue, got total=%d status=%s", total, list[0].Status)
	}
	if _, err := env.renewalService.Apply(ctx, record.ID, 2, actor); !isBusinessStatus(err, 409) {
		t.Fatalf("overdue apply expected 409, got %v", err)
	}

	// MarkOverdue 幂等：重复执行不产生额外影响。
	if n, err := env.borrowService.SyncOverdue(ctx, time.Now()); err != nil || n != 0 {
		t.Fatalf("idempotent overdue expected 0 affected, got %d err=%v", n, err)
	}
}

func TestRenewalService_OverdueAutoRejectsPendingRenewal(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}

	record, err := env.borrowService.Create(ctx, &model.BorrowRecord{
		EquipmentID:        1,
		BorrowDate:         time.Now().AddDate(0, 0, -10),
		ExpectedReturnDate: time.Now().AddDate(0, 0, -1),
	}, actor)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := env.borrowService.Approve(ctx, record.ID, approver); err != nil {
		t.Fatalf("approve: %v", err)
	}
	// 直接写入一条待审批续借（绕过服务层到期校验，模拟到期前申请、尚未审批）。
	pendingBorrowID := record.ID
	if err := env.db.Create(&model.BorrowRenewal{
		BorrowID:        record.ID,
		ExtendDays:      3,
		NewDueDate:      record.ExpectedReturnDate.AddDate(0, 0, 3),
		Status:          constants.RenewalStatusPending,
		PendingBorrowID: &pendingBorrowID,
	}).Error; err != nil {
		t.Fatalf("seed renewal: %v", err)
	}
	if _, err := env.borrowService.SyncOverdue(ctx, time.Now()); err != nil {
		t.Fatalf("sync overdue: %v", err)
	}
	renewals, err := env.renewalService.ListByBorrow(ctx, record.ID)
	if err != nil {
		t.Fatalf("list renewals: %v", err)
	}
	if len(renewals) != 1 || renewals[0].Status != constants.RenewalStatusRejected {
		t.Fatalf("pending renewal should be auto rejected, got %+v", renewals)
	}
	// 原归还日期不变。
	got, _ := env.borrowService.Get(ctx, record.ID)
	if !got.ExpectedReturnDate.Equal(record.ExpectedReturnDate) {
		t.Fatalf("overdue changed due date: %s", got.ExpectedReturnDate)
	}
}

func TestRenewalService_ReturnClosesEntry(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	actor := Actor{UserID: env.ownerID, Username: "admin", Role: "Student"}
	approver := Actor{UserID: env.ownerID, Username: "admin", Role: "Admin"}
	record := createApprovedBorrow(t, env, actor, 7)

	renewal, err := env.renewalService.Apply(ctx, record.ID, 3, actor)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := env.borrowService.Return(ctx, record.ID, time.Now(), constants.ReturnConditionGood, approver); err != nil {
		t.Fatalf("return: %v", err)
	}
	got, _ := env.borrowService.Get(ctx, record.ID)
	if got.Status != constants.BorrowStatusReturned {
		t.Fatalf("expected returned, got %s", got.Status)
	}
	// 归还后续借入口关闭。
	if _, err := env.renewalService.Apply(ctx, record.ID, 2, actor); !isBusinessStatus(err, 409) {
		t.Fatalf("returned apply expected 409, got %v", err)
	}
	renewals, _ := env.renewalService.ListByBorrow(ctx, record.ID)
	if len(renewals) != 1 || renewals[0].Status != constants.RenewalStatusRejected {
		t.Fatalf("pending renewal should be closed on return, got %+v", renewals)
	}
	if err := env.renewalService.Approve(ctx, renewal.ID, approver); !isBusinessStatus(err, 409) {
		t.Fatalf("approve after return expected 409, got %v", err)
	}
}
