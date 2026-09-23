package router

import (
	"github.com/gin-gonic/gin"
	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/handler"
	"github.com/medasset/medasset/internal/middleware"
)

// registerMaintenancePolicyRoutes 按设备类别配置的保养周期策略路由。
func registerMaintenancePolicyRoutes(g *gin.RouterGroup, h *handler.MaintenancePolicyHandler) {
	policy := g.Group("/maintenance-policies")
	{
		policy.GET("", h.List)
		policy.PUT("", middleware.RequireRoles(constants.RoleDeviceAdmin, constants.RoleSuperAdmin), h.Update)
	}
}
