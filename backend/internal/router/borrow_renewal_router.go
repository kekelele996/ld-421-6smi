package router

import (
	"github.com/gin-gonic/gin"
	"github.com/labequipment/lab-equipment/internal/middleware"
)

func registerBorrowRenewalRoutes(group *gin.RouterGroup, deps Dependencies) {
	renewals := group.Group("/renewals")
	{
		renewals.GET("", deps.BorrowRenewalHandler.List)
	}
	// 借用人在指定借用下提交续借申请、查看该笔借用的续借历史。
	borrowRenewals := group.Group("/borrows/:id/renewals")
	{
		borrowRenewals.GET("", deps.BorrowRenewalHandler.ListByBorrow)
		borrowRenewals.POST("", deps.BorrowRenewalHandler.Apply)
	}
	review := group.Group("/renewals")
	review.Use(middleware.RequireRoles("Admin", "LabManager"))
	{
		review.POST("/:id/approve", deps.BorrowRenewalHandler.Approve)
		review.POST("/:id/reject", deps.BorrowRenewalHandler.Reject)
	}
}
