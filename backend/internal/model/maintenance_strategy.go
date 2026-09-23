package model

import "time"

// MaintenanceStrategy 按设备类别配置的保养周期策略。
// 同一设备类别下每种保养类型（daily/weekly/monthly/yearly）唯一一条策略；
// enabled=false 时生成计划将跳过该类别对应的该类保养。
type MaintenanceStrategy struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Category     string    `gorm:"size:64;not null;uniqueIndex:uniq_category_type;index" json:"category"`
	Type         string    `gorm:"size:32;not null;uniqueIndex:uniq_category_type" json:"type"`
	IntervalDays int       `gorm:"not null" json:"interval_days"`
	Enabled      bool      `gorm:"not null;index" json:"enabled"`
	Remark       string    `gorm:"size:256" json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
