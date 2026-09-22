package dto

import "time"

// DashboardStatsResponse 仪表盘统计返回。
type DashboardStatsResponse struct {
	StatusDistribution  map[string]int64 `json:"statusDistribution"`
	BorrowStatusCounts  map[string]int64 `json:"borrowStatusCounts"`
	TopBorrows          []TopBorrowItem  `json:"topBorrows"`
	ExpiringWarranty    []ExpiringItem   `json:"expiringWarranty"`
	OverdueBorrows      []OverdueItem    `json:"overdueBorrows"`
	PendingBorrows      int64            `json:"pendingBorrows"`
	PendingRenewals     int64            `json:"pendingRenewals"`
	PendingReservations int64            `json:"pendingReservations"`
	OverdueCount        int64            `json:"overdueCount"`
}

// TopBorrowItem 本月借用次数排行项。
type TopBorrowItem struct {
	EquipmentID uint   `json:"equipmentId"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Count       int64  `json:"count"`
}

// ExpiringItem 即将过保设备项。
type ExpiringItem struct {
	ID             uint       `json:"id"`
	Name           string     `json:"name"`
	Code           string     `json:"code"`
	WarrantyExpiry *time.Time `json:"warrantyExpiry"`
}

// OverdueItem 逾期借用项。
type OverdueItem struct {
	ID                 uint      `json:"id"`
	EquipmentID        uint      `json:"equipmentId"`
	EquipmentName      string    `json:"equipmentName"`
	EquipmentCode      string    `json:"equipmentCode"`
	BorrowerID         uint      `json:"borrowerId"`
	BorrowerName       string    `json:"borrowerName"`
	ExpectedReturnDate time.Time `json:"expectedReturnDate"`
	OverdueDays        int       `json:"overdueDays"`
}
