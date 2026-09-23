package dto

import (
	"time"

	"github.com/medasset/medasset/internal/model"
)

// PolicyItem 单类设备（日/周/月/年检）周期策略项。
type PolicyItem struct {
	Category        string `json:"category"`
	DailyInterval   int    `json:"daily_interval"`
	WeeklyInterval  int    `json:"weekly_interval"`
	MonthlyInterval int    `json:"monthly_interval"`
	YearlyInterval  int    `json:"yearly_interval"`
}

// UpdatePolicyReq 批量更新保养周期策略（按类别逐条 upsert）。
type UpdatePolicyReq struct {
	Items []PolicyItem `json:"items" binding:"required,min=1"`
}

// PolicyListItem 策略列表返回项，附带各周期启停状态与展示文本。
type PolicyListItem struct {
	ID              uint      `json:"id"`
	Category        string    `json:"category"`
	DailyInterval   int       `json:"daily_interval"`
	WeeklyInterval  int       `json:"weekly_interval"`
	MonthlyInterval int       `json:"monthly_interval"`
	YearlyInterval  int       `json:"yearly_interval"`
	DailyEnabled    bool      `json:"daily_enabled"`
	WeeklyEnabled   bool      `json:"weekly_enabled"`
	MonthlyEnabled  bool      `json:"monthly_enabled"`
	YearlyEnabled   bool      `json:"yearly_enabled"`
	UpdatedBy       string    `json:"updated_by"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ToPolicyListItem 将策略模型转为列表项。
func ToPolicyListItem(p *model.MaintenancePolicy) PolicyListItem {
	return PolicyListItem{
		ID:              p.ID,
		Category:        p.Category,
		DailyInterval:   p.DailyInterval,
		WeeklyInterval:  p.WeeklyInterval,
		MonthlyInterval: p.MonthlyInterval,
		YearlyInterval:  p.YearlyInterval,
		DailyEnabled:    p.DailyInterval > 0,
		WeeklyEnabled:   p.WeeklyInterval > 0,
		MonthlyEnabled:  p.MonthlyInterval > 0,
		YearlyEnabled:   p.YearlyInterval > 0,
		UpdatedBy:       p.UpdatedBy,
		UpdatedAt:       p.UpdatedAt,
	}
}
