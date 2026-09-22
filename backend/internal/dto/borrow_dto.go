package dto

import "time"

// CreateBorrowRequest 提交借用申请。
type CreateBorrowRequest struct {
	EquipmentID        uint   `json:"equipmentId" binding:"required"`
	BorrowDate         string `json:"borrowDate" binding:"required"`
	ExpectedReturnDate string `json:"expectedReturnDate" binding:"required"`
	Reason             string `json:"reason" binding:"omitempty,max=512"`
}

// ReturnBorrowRequest 确认归还。
type ReturnBorrowRequest struct {
	ActualReturnDate string `json:"actualReturnDate" binding:"required"`
	ReturnCondition  string `json:"returnCondition" binding:"required"`
}

// CreateRenewalRequest 提交续借申请。
type CreateRenewalRequest struct {
	ExtendDays int    `json:"extendDays" binding:"required,min=1,max=7"`
	Reason     string `json:"reason" binding:"omitempty,max=512"`
}

// ReviewRenewalRequest 审批续借申请（驳回时可填写意见）。
type ReviewRenewalRequest struct {
	Comment string `json:"comment" binding:"omitempty,max=512"`
}

// PendingRenewalBrief 借用记录内嵌的待审批续借申请摘要。
type PendingRenewalBrief struct {
	ID               uint      `json:"id"`
	ExtendDays       int       `json:"extendDays"`
	RequestedDueDate time.Time `json:"requestedDueDate"`
	CreatedAt        time.Time `json:"createdAt"`
}

// BorrowResponse 借用记录返回。
type BorrowResponse struct {
	ID                     uint                 `json:"id"`
	EquipmentID            uint                 `json:"equipmentId"`
	EquipmentName          string               `json:"equipmentName,omitempty"`
	EquipmentCode          string               `json:"equipmentCode,omitempty"`
	BorrowerID             uint                 `json:"borrowerId"`
	BorrowerName           string               `json:"borrowerName,omitempty"`
	BorrowDate             time.Time            `json:"borrowDate"`
	ExpectedReturnDate     time.Time            `json:"expectedReturnDate"`
	OriginalExpectedReturn time.Time            `json:"originalExpectedReturn"`
	ActualReturnDate       *time.Time           `json:"actualReturnDate"`
	Reason                 string               `json:"reason"`
	Status                 string               `json:"status"`
	ApproverID             *uint                `json:"approverId"`
	ApproverName           string               `json:"approverName,omitempty"`
	ReturnCondition        *string              `json:"returnCondition"`
	PendingRenewal         *PendingRenewalBrief `json:"pendingRenewal,omitempty"`
	CreatedAt              time.Time            `json:"createdAt"`
}

// RenewalResponse 续借申请返回。
type RenewalResponse struct {
	ID               uint       `json:"id"`
	BorrowID         uint       `json:"borrowId"`
	EquipmentID      uint       `json:"equipmentId,omitempty"`
	EquipmentName    string     `json:"equipmentName,omitempty"`
	ApplicantID      uint       `json:"applicantId"`
	ApplicantName    string     `json:"applicantName,omitempty"`
	ExtendDays       int        `json:"extendDays"`
	CurrentDueDate   time.Time  `json:"currentDueDate"`
	RequestedDueDate time.Time  `json:"requestedDueDate"`
	Reason           string     `json:"reason"`
	Status           string     `json:"status"`
	ReviewerID       *uint      `json:"reviewerId"`
	ReviewerName     string     `json:"reviewerName,omitempty"`
	ReviewComment    string     `json:"reviewComment,omitempty"`
	ReviewedAt       *time.Time `json:"reviewedAt"`
	CreatedAt        time.Time  `json:"createdAt"`
}
