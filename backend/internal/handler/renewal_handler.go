package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/labequipment/lab-equipment/internal/dto"
	"github.com/labequipment/lab-equipment/internal/middleware"
	"github.com/labequipment/lab-equipment/internal/service"
)

// RenewalHandler 续借申请处理器。
type RenewalHandler struct {
	renewalService *service.RenewalService
}

// NewRenewalHandler 构造续借处理器。
func NewRenewalHandler(renewalService *service.RenewalService) *RenewalHandler {
	return &RenewalHandler{renewalService: renewalService}
}

// Apply 借用人对指定借用提交续借申请。
func (h *RenewalHandler) Apply(c *gin.Context) {
	borrowID, valid := parseID(c)
	if !valid {
		return
	}
	var req dto.CreateRenewalRequest
	if !bindJSON(c, &req) {
		return
	}
	renewal, err := h.renewalService.Apply(c.Request.Context(), borrowID, req.ExtendDays, middleware.GetActor(c))
	if err != nil {
		failError(c, err)
		return
	}
	ok(c, mapRenewal(*renewal))
}

// ListByBorrow 查询某笔借用的续借历史。
func (h *RenewalHandler) ListByBorrow(c *gin.Context) {
	borrowID, valid := parseID(c)
	if !valid {
		return
	}
	list, err := h.renewalService.ListByBorrow(c.Request.Context(), borrowID)
	if err != nil {
		failError(c, err)
		return
	}
	ok(c, mapRenewalList(list))
}

// Approve 管理员批准续借。
func (h *RenewalHandler) Approve(c *gin.Context) {
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

// Reject 管理员驳回续借。
func (h *RenewalHandler) Reject(c *gin.Context) {
	id, valid := parseID(c)
	if !valid {
		return
	}
	var req dto.RejectRenewalRequest
	if c.Request.ContentLength > 0 && !bindJSON(c, &req) {
		return
	}
	if err := h.renewalService.Reject(c.Request.Context(), id, req.Reason, middleware.GetActor(c)); err != nil {
		failError(c, err)
		return
	}
	ok(c, nil)
}
