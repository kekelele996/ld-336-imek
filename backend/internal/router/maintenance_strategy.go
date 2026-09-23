package router

import (
	"github.com/gin-gonic/gin"
	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/handler"
	"github.com/medasset/medasset/internal/middleware"
)

// registerMaintenanceStrategyRoutes 保养周期策略路由。
func registerMaintenanceStrategyRoutes(g *gin.RouterGroup, h *handler.MaintenanceStrategyHandler) {
	st := g.Group("/maintenance-strategies")
	{
		st.GET("", h.List)
		st.POST("", middleware.RequireRoles(constants.RoleDeviceAdmin, constants.RoleSuperAdmin), h.Create)
		st.PUT("/:id", middleware.RequireRoles(constants.RoleDeviceAdmin, constants.RoleSuperAdmin), h.Update)
	}
}
