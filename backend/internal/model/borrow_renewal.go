package model

import (
	"time"

	"github.com/labequipment/lab-equipment/internal/constants"
)

// BorrowRenewal 借用续借申请。每笔借用同一时间最多存在一条待审批申请。
type BorrowRenewal struct {
	Base
	BorrowID     uint                    `gorm:"index;not null" json:"borrowId"`
	ExtendDays   int                     `gorm:"not null" json:"extendDays"`
	NewDueDate   time.Time               `json:"newDueDate"`
	Status       constants.RenewalStatus `gorm:"size:32;index;not null;default:Pending" json:"status"`
	ReviewerID   *uint                   `json:"reviewerId"`
	ReviewReason string                  `gorm:"size:512" json:"reviewReason"`
	// PendingBorrowID 仅待审批申请持有借用 ID（唯一索引保证每笔借用仅一条待审批申请）。
	PendingBorrowID *uint         `gorm:"uniqueIndex" json:"-"`
	Borrow          *BorrowRecord `gorm:"foreignKey:BorrowID" json:"borrow,omitempty"`
	Reviewer        *User         `gorm:"foreignKey:ReviewerID" json:"reviewer,omitempty"`
}

func (BorrowRenewal) TableName() string { return "borrow_renewals" }
