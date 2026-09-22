package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
)

// DashboardService 仪表盘统计服务。
type DashboardService struct {
	equipmentRepo   repository.EquipmentRepository
	borrowRepo      repository.BorrowRepository
	renewalRepo     repository.BorrowRenewalRepository
	reservationRepo repository.ReservationRepository
	borrowService   *BorrowService
	logger          *slog.Logger
}

// NewDashboardService 构造仪表盘服务。
func NewDashboardService(
	equipmentRepo repository.EquipmentRepository,
	borrowRepo repository.BorrowRepository,
	renewalRepo repository.BorrowRenewalRepository,
	reservationRepo repository.ReservationRepository,
	borrowService *BorrowService,
	logger *slog.Logger,
) *DashboardService {
	return &DashboardService{
		equipmentRepo:   equipmentRepo,
		borrowRepo:      borrowRepo,
		renewalRepo:     renewalRepo,
		reservationRepo: reservationRepo,
		borrowService:   borrowService,
		logger:          logger,
	}
}

// Stats 聚合仪表盘所需统计。
func (s *DashboardService) Stats(ctx context.Context) (*DashboardStats, error) {
	// 总览与借用列表状态同步：先将到期未归还的借用刷新为逾期。
	if err := s.borrowService.SweepOverdue(ctx); err != nil {
		s.logger.WarnContext(ctx, "sweep overdue borrows on dashboard failed", "error", err)
	}
	statusMap, err := s.equipmentRepo.CountByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard status distribution: %w", err)
	}
	topBorrows, err := s.borrowRepo.TopEquipmentThisMonth(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("dashboard top borrows: %w", err)
	}
	expiring, err := s.equipmentRepo.ListExpiringWarranty(ctx, 30)
	if err != nil {
		return nil, fmt.Errorf("dashboard expiring warranty: %w", err)
	}
	pendingBorrows, err := s.borrowRepo.CountPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard pending borrows: %w", err)
	}
	pendingRenewals, err := s.renewalRepo.CountPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard pending renewals: %w", err)
	}
	overdueBorrows, err := s.borrowRepo.CountOverdue(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard overdue borrows: %w", err)
	}
	overdueList, err := s.borrowRepo.ListOverdue(ctx, 10)
	if err != nil {
		return nil, fmt.Errorf("dashboard overdue borrow list: %w", err)
	}
	borrowStatusCounts, err := s.borrowRepo.CountByStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard borrow status counts: %w", err)
	}
	pendingReservations, err := s.reservationRepo.CountPending(ctx)
	if err != nil {
		return nil, fmt.Errorf("dashboard pending reservations: %w", err)
	}
	statusDistribution := make(map[string]int64, len(statusMap))
	for k, v := range statusMap {
		statusDistribution[string(k)] = v
	}
	borrowStatusDistribution := make(map[string]int64, len(borrowStatusCounts))
	for k, v := range borrowStatusCounts {
		borrowStatusDistribution[string(k)] = v
	}
	return &DashboardStats{
		StatusDistribution:  statusDistribution,
		BorrowStatusCounts:  borrowStatusDistribution,
		TopBorrows:          topBorrows,
		ExpiringWarranty:    expiring,
		OverdueBorrows:      overdueList,
		PendingBorrows:      pendingBorrows,
		PendingRenewals:     pendingRenewals,
		PendingReservations: pendingReservations,
		OverdueCount:        overdueBorrows,
	}, nil
}

// DashboardStats 仪表盘统计结果。
type DashboardStats struct {
	StatusDistribution  map[string]int64
	BorrowStatusCounts  map[string]int64
	TopBorrows          []repository.BorrowTopStat
	ExpiringWarranty    []model.Equipment
	OverdueBorrows      []model.BorrowRecord
	PendingBorrows      int64
	PendingRenewals     int64
	PendingReservations int64
	OverdueCount        int64
}

// OverdueDays 计算借用逾期天数（至少为 1）。
func OverdueDays(expectedReturnDate time.Time, now time.Time) int {
	days := int(now.Sub(expectedReturnDate).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}
