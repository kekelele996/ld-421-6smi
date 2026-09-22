package router

import (
	"io"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/labequipment/lab-equipment/database/migrations"
	"github.com/labequipment/lab-equipment/internal/config"
	"github.com/labequipment/lab-equipment/internal/handler"
	"github.com/labequipment/lab-equipment/internal/repository"
	"github.com/labequipment/lab-equipment/internal/service"
	"gorm.io/gorm"
)

// TestNewRouter_RenewalRoutesRegistered 验证借用与续借路由可共存注册（Gin 静态/参数段冲突会 panic）。
func TestNewRouter_RenewalRoutesRegistered(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migrations.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	equipmentRepo := repository.NewEquipmentRepository(db)
	borrowRepo := repository.NewBorrowRepository(db)
	renewalRepo := repository.NewBorrowRenewalRepository(db)
	reservationRepo := repository.NewReservationRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)

	auditService := service.NewAuditService(auditRepo, logger)
	borrowService := service.NewBorrowService(borrowRepo, equipmentRepo, auditService, logger)
	renewalService := service.NewBorrowRenewalService(renewalRepo, borrowRepo, auditService, logger)
	dashboardService := service.NewDashboardService(equipmentRepo, borrowRepo, renewalRepo, reservationRepo, borrowService, logger)

	deps := Dependencies{
		BorrowHandler:        handler.NewBorrowHandler(borrowService),
		BorrowRenewalHandler: handler.NewBorrowRenewalHandler(renewalService),
		DashboardHandler:     handler.NewDashboardHandler(dashboardService),
		AuthService:          service.NewAuthService(repository.NewUserRepository(db), config.JWTConfig{Secret: "test-secret", ExpireHours: 24}, logger),
		AuditService:         auditService,
		Logger:               logger,
	}

	engine := NewRouter(deps)
	paths := map[string]bool{}
	for _, route := range engine.Routes() {
		paths[route.Method+" "+route.Path] = true
	}
	expected := []string{
		"POST /api/v1/borrows/:id/renewals",
		"GET /api/v1/borrows/:id/renewals",
		"GET /api/v1/renewals",
		"POST /api/v1/renewals/:id/approve",
		"POST /api/v1/renewals/:id/reject",
		"GET /api/v1/dashboard/stats",
	}
	for _, p := range expected {
		if !paths[p] {
			t.Errorf("route not registered: %s", p)
		}
	}
}
