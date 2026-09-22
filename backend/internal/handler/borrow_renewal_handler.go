package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/labequipment/lab-equipment/internal/constants"
	"github.com/labequipment/lab-equipment/internal/dto"
	"github.com/labequipment/lab-equipment/internal/middleware"
	"github.com/labequipment/lab-equipment/internal/repository"
	"github.com/labequipment/lab-equipment/internal/service"
)

// BorrowRenewalHandler 续借申请处理器。
type BorrowRenewalHandler struct {
	renewalService *service.BorrowRenewalService
}

// NewBorrowRenewalHandler 构造续借处理器。
func NewBorrowRenewalHandler(renewalService *service.BorrowRenewalService) *BorrowRenewalHandler {
	return &BorrowRenewalHandler{renewalService: renewalService}
}

// List 分页查询续借申请。
func (h *BorrowRenewalHandler) List(c *gin.Context) {
	page, pageSize := parsePage(c)
	borrowID := uint(0)
	if v := c.Query("borrow_id"); v != "" {
		id, ok := parseUintParam(c, v)
		if !ok {
			return
		}
		borrowID = id
	}
	filter := repository.RenewalFilter{
		Status:     constants.RenewalStatus(c.Query("status")),
		BorrowID:   borrowID,
		Pagination: repository.Pagination{Page: page, PageSize: pageSize},
	}
	list, total, err := h.renewalService.List(c.Request.Context(), filter)
	if err != nil {
		failError(c, err)
		return
	}
	items := make([]dto.RenewalResponse, 0, len(list))
	for _, renewal := range list {
		items = append(items, mapRenewal(renewal))
	}
	ok(c, dto.PageResult{List: items, Total: total, Page: page, PageSize: pageSize})
}

// ListByBorrow 查询单笔借用的续借历史。
func (h *BorrowRenewalHandler) ListByBorrow(c *gin.Context) {
	borrowID, valid := parseID(c)
	if !valid {
		return
	}
	list, err := h.renewalService.ListByBorrow(c.Request.Context(), borrowID)
	if err != nil {
		failError(c, err)
		return
	}
	items := make([]dto.RenewalResponse, 0, len(list))
	for _, renewal := range list {
		items = append(items, mapRenewal(renewal))
	}
	ok(c, items)
}

// Apply 提交续借申请。
func (h *BorrowRenewalHandler) Apply(c *gin.Context) {
	borrowID, valid := parseID(c)
	if !valid {
		return
	}
	var req dto.CreateRenewalRequest
	if !bindJSON(c, &req) {
		return
	}
	renewal, err := h.renewalService.Apply(c.Request.Context(), borrowID, req.ExtendDays, req.Reason, middleware.GetActor(c))
	if err != nil {
		failError(c, err)
		return
	}
	ok(c, mapRenewal(*renewal))
}

// Approve 批准续借。
func (h *BorrowRenewalHandler) Approve(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	if err := h.renewalService.Approve(c.Request.Context(), id, middleware.GetActor(c)); err != nil {
		failError(c, err)
		return
	}
	ok(c, nil)
}

// Reject 驳回续借。
func (h *BorrowRenewalHandler) Reject(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	var req dto.ReviewRenewalRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, 400, 40000, "请求参数格式错误")
			return
		}
	}
	if err := h.renewalService.Reject(c.Request.Context(), id, req.Comment, middleware.GetActor(c)); err != nil {
		failError(c, err)
		return
	}
	ok(c, nil)
}
