package model

import (
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
)

// BorrowRenewal 借用续借申请实体。每笔借用至多存在一条待审批申请。
type BorrowRenewal struct {
	Base
	BorrowID         uint                    `gorm:"index;not null" json:"borrowId"`
	ApplicantID      uint                    `gorm:"index;not null" json:"applicantId"`
	ExtendDays       int                     `gorm:"not null" json:"extendDays"`
	CurrentDueDate   time.Time               `json:"currentDueDate"`
	RequestedDueDate time.Time               `json:"requestedDueDate"`
	Reason           string                  `gorm:"size:512" json:"reason"`
	Status           constants.RenewalStatus `gorm:"size:32;index;not null;default:Pending" json:"status"`
	ReviewerID       *uint                   `json:"reviewerId"`
	ReviewComment    string                  `gorm:"size:512" json:"reviewComment"`
	ReviewedAt       *time.Time              `json:"reviewedAt"`
	// PendingBorrowID 仅在申请待审批时等于 BorrowID，其余状态为 NULL；
	// 配合唯一索引保证每笔借用至多一条待审批续借申请。
	PendingBorrowID *uint         `gorm:"uniqueIndex" json:"-"`
	Borrow          *BorrowRecord `gorm:"foreignKey:BorrowID" json:"borrow,omitempty"`
	Applicant       *User         `gorm:"foreignKey:ApplicantID" json:"applicant,omitempty"`
	Reviewer        *User         `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}

func (BorrowRenewal) TableName() string { return "borrow_renewals" }
