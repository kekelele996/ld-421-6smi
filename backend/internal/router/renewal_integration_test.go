package router

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/labequipment/lab-equipment/database/migrations"
	"github.com/labequipment/lab-equipment/internal/config"
	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/handler"
	"github.com/labequipment/lab-equipment/internal/model"
	"github.com/labequipment/lab-equipment/internal/repository"
	"github.com/labequipment/lab-equipment/internal/service"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type integrationEnv struct {
	handler http.Handler
	db      *gorm.DB
}

type apiResponse struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func setupIntegration(t *testing.T) *integrationEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := migrations.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	role := model.Role{Code: "Admin", Name: "管理员"}
	if err := db.Create(&role).Error; err != nil {
		t.Fatalf("create role: %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pass123"), bcrypt.DefaultCost)
	user := model.User{Username: "admin", PasswordHash: string(hash), Name: "管理员", RoleID: role.ID, Active: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	category := model.EquipmentCategory{Name: "仪器"}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}
	equipment := model.Equipment{Name: "设备A", Code: "EQ-A", CategoryID: category.ID, OwnerID: user.ID, Status: constants.AssetStatusAvailable}
	if err := db.Create(&equipment).Error; err != nil {
		t.Fatalf("create equipment: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	categoryRepo := repository.NewCategoryRepository(db)
	equipmentRepo := repository.NewEquipmentRepository(db)
	borrowRepo := repository.NewBorrowRepository(db)
	renewalRepo := repository.NewBorrowRenewalRepository(db)
	reservationRepo := repository.NewReservationRepository(db)
	maintenanceRepo := repository.NewMaintenanceRepository(db)
	auditRepo := repository.NewAuditLogRepository(db)

	auditService := service.NewAuditService(auditRepo, logger)
	authService := service.NewAuthService(userRepo, config.JWTConfig{Secret: "test-secret", ExpireHours: 24}, logger)
	equipmentService := service.NewEquipmentService(equipmentRepo, categoryRepo, userRepo, auditService, logger)
	borrowService := service.NewBorrowService(borrowRepo, equipmentRepo, auditService, logger)
	renewalService := service.NewBorrowRenewalService(renewalRepo, borrowRepo, auditService, logger)
	maintenanceService := service.NewMaintenanceService(maintenanceRepo, equipmentRepo, auditService, logger)
	reservationService := service.NewReservationService(reservationRepo, equipmentRepo, auditService, logger)
	dashboardService := service.NewDashboardService(equipmentRepo, borrowRepo, renewalRepo, reservationRepo, borrowService, logger)

	deps := Dependencies{
		AuthHandler:          handler.NewAuthHandler(authService),
		EquipmentHandler:     handler.NewEquipmentHandler(equipmentService),
		BorrowHandler:        handler.NewBorrowHandler(borrowService),
		BorrowRenewalHandler: handler.NewBorrowRenewalHandler(renewalService),
		MaintenanceHandler:   handler.NewMaintenanceHandler(maintenanceService),
		ReservationHandler:   handler.NewReservationHandler(reservationService),
		DashboardHandler:     handler.NewDashboardHandler(dashboardService),
		AuthService:          authService,
		AuditService:         auditService,
		Logger:               logger,
	}
	return &integrationEnv{handler: NewRouter(deps), db: db}
}

func (e *integrationEnv) do(t *testing.T, method, path, token string, body any) (int, apiResponse) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	var resp apiResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return rec.Code, resp
}

func loginToken(t *testing.T, env *integrationEnv) string {
	t.Helper()
	status, resp := env.do(t, http.MethodPost, "/api/v1/auth/login", "", map[string]string{"username": "admin", "password": "pass123"})
	if status != http.StatusOK {
		t.Fatalf("login status=%d resp=%s", status, resp.Message)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	return data.Token
}

func TestIntegration_RenewalFlowOverHTTP(t *testing.T) {
	env := setupIntegration(t)
	token := loginToken(t, env)

	// 提交借用：借用日 10 天前，预计 5 天后归还。
	borrowPayload := map[string]any{
		"equipmentId":        1,
		"borrowDate":         time.Now().AddDate(0, 0, -10).Format("2006-01-02"),
		"expectedReturnDate": time.Now().AddDate(0, 0, 5).Format("2006-01-02"),
	}
	status, resp := env.do(t, http.MethodPost, "/api/v1/borrows", token, borrowPayload)
	if status != http.StatusOK {
		t.Fatalf("create borrow: status=%d msg=%s", status, resp.Message)
	}
	var borrow struct {
		ID uint `json:"id"`
	}
	_ = json.Unmarshal(resp.Data, &borrow)

	// 审批借用。
	if status, _ := env.do(t, http.MethodPost, "/api/v1/borrows/"+itoa(borrow.ID)+"/approve", token, nil); status != http.StatusOK {
		t.Fatalf("approve borrow status=%d", status)
	}

	// 申请续借 3 天。
	status, resp = env.do(t, http.MethodPost, "/api/v1/borrows/"+itoa(borrow.ID)+"/renewals", token,
		map[string]any{"extendDays": 3, "reason": "实验未完"})
	if status != http.StatusOK {
		t.Fatalf("apply renewal: status=%d msg=%s", status, resp.Message)
	}
	var renewal struct {
		ID               uint   `json:"id"`
		RequestedDueDate string `json:"requestedDueDate"`
		Status           string `json:"status"`
	}
	_ = json.Unmarshal(resp.Data, &renewal)
	if renewal.Status != "Pending" {
		t.Fatalf("renewal should be pending, got %s", renewal.Status)
	}

	// 重复申请被拒（每笔借用只保留一条待审批）。
	if status, resp := env.do(t, http.MethodPost, "/api/v1/borrows/"+itoa(borrow.ID)+"/renewals", token,
		map[string]any{"extendDays": 2}); status != http.StatusConflict {
		t.Fatalf("duplicate renewal should be 409, got %d msg=%s", status, resp.Message)
	}

	// 延长期限越界被拒。
	if status, _ := env.do(t, http.MethodPost, "/api/v1/borrows/"+itoa(borrow.ID)+"/renewals", token,
		map[string]any{"extendDays": 8}); status != http.StatusUnprocessableEntity {
		t.Fatalf("extendDays=8 should be 422, got %d", status)
	}

	// 借用列表内嵌待审批续借。
	_, listResp := env.do(t, http.MethodGet, "/api/v1/borrows?page=1&page_size=10", token, nil)
	var page struct {
		List []struct {
			ID             uint `json:"id"`
			PendingRenewal *struct {
				ID uint `json:"id"`
			} `json:"pendingRenewal"`
		} `json:"list"`
	}
	_ = json.Unmarshal(listResp.Data, &page)
	found := false
	for _, item := range page.List {
		if item.ID == borrow.ID {
			found = item.PendingRenewal != nil && item.PendingRenewal.ID == renewal.ID
		}
	}
	if !found {
		t.Fatalf("pending renewal should be embedded in borrow list")
	}

	// 并发批准续借，仅一个成功。
	var wg sync.WaitGroup
	results := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			code, _ := env.do(t, http.MethodPost, "/api/v1/renewals/"+itoa(renewal.ID)+"/approve", token, nil)
			results[idx] = code
		}(i)
	}
	wg.Wait()
	if results[0] == results[1] {
		t.Fatalf("concurrent approve must succeed once: results=%v", results)
	}
	successCount := 0
	for _, code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly one successful approve, results=%v", results)
	}

	// 刷新后状态与日期一致：预计归还延长 3 天，原始到期保留。
	_, detailResp := env.do(t, http.MethodGet, "/api/v1/borrows/"+itoa(borrow.ID), token, nil)
	var detail struct {
		ExpectedReturnDate     string `json:"expectedReturnDate"`
		OriginalExpectedReturn string `json:"originalExpectedReturn"`
		Status                 string `json:"status"`
		PendingRenewal         *struct {
			ID uint `json:"id"`
		} `json:"pendingRenewal"`
	}
	_ = json.Unmarshal(detailResp.Data, &detail)
	wantDue := time.Now().AddDate(0, 0, 8)
	gotDue, _ := time.Parse("2006-01-02T15:04:05Z", detail.ExpectedReturnDate)
	if gotDue.Format("2006-01-02") != wantDue.Format("2006-01-02") {
		t.Fatalf("due should be extended to %s, got %s", wantDue.Format("2006-01-02"), detail.ExpectedReturnDate)
	}
	if detail.OriginalExpectedReturn[:10] != time.Now().AddDate(0, 0, 5).Format("2006-01-02") {
		t.Fatalf("original due must be preserved, got %s", detail.OriginalExpectedReturn)
	}
	if detail.Status != "Approved" || detail.PendingRenewal != nil {
		t.Fatalf("after approve status=%s pending=%v", detail.Status, detail.PendingRenewal)
	}
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}
