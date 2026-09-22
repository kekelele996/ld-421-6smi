package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/labequipment/lab-equipment/internal/dto"
	"github.com/labequipment/lab-equipment/internal/service"
)

// DashboardHandler 仪表盘处理器。
type DashboardHandler struct {
	dashboardService *service.DashboardService
}

// NewDashboardHandler 构造仪表盘处理器。
func NewDashboardHandler(dashboardService *service.DashboardService) *DashboardHandler {
	return &DashboardHandler{dashboardService: dashboardService}
}

// Stats 获取仪表盘统计。
func (h *DashboardHandler) Stats(c *gin.Context) {
	stats, err := h.dashboardService.Stats(c.Request.Context())
	if err != nil {
		failError(c, err)
		return
	}
	topBorrows := make([]dto.TopBorrowItem, 0, len(stats.TopBorrows))
	for _, item := range stats.TopBorrows {
		topBorrows = append(topBorrows, dto.TopBorrowItem{
			EquipmentID: item.EquipmentID,
			Name:        item.Name,
			Code:        item.Code,
			Count:       item.Count,
		})
	}
	expiring := make([]dto.ExpiringItem, 0, len(stats.ExpiringWarranty))
	for _, equipment := range stats.ExpiringWarranty {
		expiring = append(expiring, dto.ExpiringItem{
			ID:             equipment.ID,
			Name:           equipment.Name,
			Code:           equipment.Code,
			WarrantyExpiry: equipment.WarrantyExpiry,
		})
	}
	now := time.Now()
	overdue := make([]dto.OverdueItem, 0, len(stats.OverdueBorrows))
	for _, record := range stats.OverdueBorrows {
		equipmentName := ""
		equipmentCode := ""
		if record.Equipment != nil {
			equipmentName = record.Equipment.Name
			equipmentCode = record.Equipment.Code
		}
		borrowerName := ""
		if record.Borrower != nil {
			borrowerName = record.Borrower.Name
		}
		overdue = append(overdue, dto.OverdueItem{
			ID:                 record.ID,
			EquipmentID:        record.EquipmentID,
			EquipmentName:      equipmentName,
			EquipmentCode:      equipmentCode,
			BorrowerID:         record.BorrowerID,
			BorrowerName:       borrowerName,
			ExpectedReturnDate: record.ExpectedReturnDate,
			OverdueDays:        service.OverdueDays(record.ExpectedReturnDate, now),
		})
	}
	ok(c, dto.DashboardStatsResponse{
		StatusDistribution:  stats.StatusDistribution,
		BorrowStatusCounts:  stats.BorrowStatusCounts,
		TopBorrows:          topBorrows,
		ExpiringWarranty:    expiring,
		OverdueBorrows:      overdue,
		PendingBorrows:      stats.PendingBorrows,
		PendingRenewals:     stats.PendingRenewals,
		PendingReservations: stats.PendingReservations,
		OverdueCount:        stats.OverdueCount,
	})
}
