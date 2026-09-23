package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/middleware"
	"github.com/medasset/medasset/internal/service"
	"github.com/medasset/medasset/internal/util"
)

// MaintenancePolicyHandler 按设备类别配置的保养周期策略处理器。
type MaintenancePolicyHandler struct {
	svc *service.MaintenancePolicyService
}

func NewMaintenancePolicyHandler(svc *service.MaintenancePolicyService) *MaintenancePolicyHandler {
	return &MaintenancePolicyHandler{svc: svc}
}

// List 查询保养周期策略列表。
func (h *MaintenancePolicyHandler) List(c *gin.Context) {
	operator := middleware.CurrentUser(c)
	items, err := h.svc.List(operator.Username)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, items)
}

// Update 批量保存保养周期策略（停用/调整周期）。
func (h *MaintenancePolicyHandler) Update(c *gin.Context) {
	var req dto.UpdatePolicyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(util.NewAppError(http.StatusBadRequest, "保养周期策略参数不合法", err))
		return
	}
	operator := middleware.CurrentUser(c)
	items, err := h.svc.Update(&req, operator.Username)
	if err != nil {
		c.Error(err)
		return
	}
	util.OK(c, items)
}
