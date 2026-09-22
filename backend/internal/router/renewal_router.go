package router

import (
	"github.com/gin-gonic/gin"
	"github.com/labequipment/lab-equipment/internal/middleware"
)

func registerRenewalRoutes(group *gin.RouterGroup, deps Dependencies) {
	// 借用人发起与查看续借（服务层校验仅本人可申请）。
	group.POST("/borrows/:id/renewals", deps.RenewalHandler.Apply)
	group.GET("/borrows/:id/renewals", deps.RenewalHandler.ListByBorrow)

	review := group.Group("/renewals")
	review.Use(middleware.RequireRoles("Admin", "LabManager"))
	{
		review.POST("/:id/approve", deps.RenewalHandler.Approve)
		review.POST("/:id/reject", deps.RenewalHandler.Reject)
	}
}
