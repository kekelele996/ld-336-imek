package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/middleware"
	"github.com/medasset/medasset/internal/service"
	"github.com/medasset/medasset/internal/util"
)

// MaintenanceStrategyHandler 保养周期策略处理器。
type MaintenanceStrategyHandler struct {
	svc *service.MaintenanceStrategyService
}

func NewMaintenanceStrategyHandler(svc *service.MaintenanceStrategyService) *MaintenanceStrategyHandler {
	return &MaintenanceStrategyHandler{svc: svc}
}

// List 周期策略列表。
func (h *MaintenanceStrategyHandler) List(c *gin.Context) {
	category := c.Query("category")
	var enabled *bool
	if v := c.Query("enabled"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			c.Error(util.NewAppError(http.StatusBadRequest, "enabled 参数不合法: enabled="+v, err))
			return
		}
		enabled = &b
	}
	items, err := h.svc.List(category, enabled)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, items)
}

// Create 新建周期策略。
func (h *MaintenanceStrategyHandler) Create(c *gin.Context) {
	var req dto.CreateMaintenanceStrategyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, "保养周期策略参数不合法", err))
		return
	}
	operator := middleware.CurrentUser(c)
	m, err := h.svc.Create(&req, operator.Username)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, m)
}

// Update 调整周期策略（周期/停用启用）。
func (h *MaintenanceStrategyHandler) Update(c *gin.Context) {
	var p dto.IDParam
	if err := c.ShouldBindUri(&p); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, "策略ID不合法", err))
		return
	}
	var req dto.UpdateMaintenanceStrategyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, "保养周期策略参数不合法", err))
		return
	}
	operator := middleware.CurrentUser(c)
	m, err := h.svc.Update(p.ID, &req, operator.Username)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, m)
}
