package dto

import "github.com/medasset/medasset/internal/model"

// CreateMaintenanceStrategyReq 新建保养周期策略请求。
type CreateMaintenanceStrategyReq struct {
	Category     string `json:"category" binding:"required,max=64"`
	Type         string `json:"type" binding:"required,oneof=daily weekly monthly yearly"`
	IntervalDays int    `json:"interval_days" binding:"required,min=1,max=3650"`
	Enabled      *bool  `json:"enabled"`
	Remark       string `json:"remark" binding:"omitempty,max=256"`
}

// UpdateMaintenanceStrategyReq 调整保养周期策略请求（周期调整/停用启用）。
type UpdateMaintenanceStrategyReq struct {
	IntervalDays *int   `json:"interval_days" binding:"omitempty,min=1,max=3650"`
	Enabled      *bool  `json:"enabled"`
	Remark       string `json:"remark" binding:"omitempty,max=256"`
}

// MaintenanceStrategyItem 周期策略返回项。
type MaintenanceStrategyItem struct {
	ID           uint   `json:"id"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	IntervalDays int    `json:"interval_days"`
	Enabled      bool   `json:"enabled"`
	Remark       string `json:"remark"`
}

// ToMaintenanceStrategyItem 模型转 DTO。
func ToMaintenanceStrategyItem(s *model.MaintenanceStrategy) MaintenanceStrategyItem {
	return MaintenanceStrategyItem{
		ID:           s.ID,
		Category:     s.Category,
		Type:         s.Type,
		IntervalDays: s.IntervalDays,
		Enabled:      s.Enabled,
		Remark:       s.Remark,
	}
}

// ToMaintenanceStrategyList 批量转换。
func ToMaintenanceStrategyList(list []model.MaintenanceStrategy) []MaintenanceStrategyItem {
	items := make([]MaintenanceStrategyItem, 0, len(list))
	for i := range list {
		items = append(items, ToMaintenanceStrategyItem(&list[i]))
	}
	return items
}
