package model

import "time"

// MaintenancePolicy 按设备类别（Device.Category）配置的保养周期策略。
// 日检/周检/月检/年检四类计划各自一个间隔（天），间隔为 0 表示停用该类计划。
type MaintenancePolicy struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Category        string    `gorm:"size:64;uniqueIndex;not null" json:"category"`
	DailyInterval   int       `gorm:"not null" json:"daily_interval"`
	WeeklyInterval  int       `gorm:"not null" json:"weekly_interval"`
	MonthlyInterval int       `gorm:"not null" json:"monthly_interval"`
	YearlyInterval  int       `gorm:"not null" json:"yearly_interval"`
	UpdatedBy       string    `gorm:"size:64" json:"updated_by"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
