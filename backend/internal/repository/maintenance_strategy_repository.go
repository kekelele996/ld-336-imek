package repository

import (
	"errors"

	"github.com/medasset/medasset/internal/model"
	"gorm.io/gorm"
)

// MaintenanceStrategyRepository 保养周期策略仓储。
type MaintenanceStrategyRepository struct {
	db *gorm.DB
}

func NewMaintenanceStrategyRepository(db *gorm.DB) *MaintenanceStrategyRepository {
	return &MaintenanceStrategyRepository{db: db}
}

// DB 返回底层数据库句柄（供 service 层开启事务）。
func (r *MaintenanceStrategyRepository) DB() *gorm.DB { return r.db }

// Create 创建策略。
func (r *MaintenanceStrategyRepository) Create(s *model.MaintenanceStrategy) error {
	return r.db.Create(s).Error
}

// FindByID 按 ID 查询。
func (r *MaintenanceStrategyRepository) FindByID(id uint) (*model.MaintenanceStrategy, error) {
	var s model.MaintenanceStrategy
	err := r.db.First(&s, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &s, err
}

// FindByCategoryType 按设备类别与保养类型查询唯一策略。
func (r *MaintenanceStrategyRepository) FindByCategoryType(category, mType string) (*model.MaintenanceStrategy, error) {
	var s model.MaintenanceStrategy
	err := r.db.Where("category = ? AND type = ?", category, mType).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &s, err
}

// FindByCategoryTypeTx 在指定事务内按设备类别与保养类型查询策略（完成工单顺延下一期复用）。
func (r *MaintenanceStrategyRepository) FindByCategoryTypeTx(tx *gorm.DB, category, mType string) (*model.MaintenanceStrategy, error) {
	var s model.MaintenanceStrategy
	err := tx.Where("category = ? AND type = ?", category, mType).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &s, err
}

// List 查询策略列表，可按设备类别/启用状态过滤（enabled 为 nil 时不过滤）。
func (r *MaintenanceStrategyRepository) List(category string, enabled *bool) ([]model.MaintenanceStrategy, error) {
	q := r.db.Model(&model.MaintenanceStrategy{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if enabled != nil {
		q = q.Where("enabled = ?", *enabled)
	}
	var list []model.MaintenanceStrategy
	err := q.Order("category ASC, CASE type WHEN 'daily' THEN 1 WHEN 'weekly' THEN 2 WHEN 'monthly' THEN 3 WHEN 'yearly' THEN 4 ELSE 5 END, id ASC").Find(&list).Error
	return list, err
}

// ListEnabled 查询全部启用策略（生成计划复用）。
func (r *MaintenanceStrategyRepository) ListEnabled() ([]model.MaintenanceStrategy, error) {
	var list []model.MaintenanceStrategy
	err := r.db.Where("enabled = ?", true).Order("category ASC, id ASC").Find(&list).Error
	return list, err
}

// ExistsByCategoryType 判断某类别+类型策略是否已存在（排除自身 ID，为 0 时不排除）。
func (r *MaintenanceStrategyRepository) ExistsByCategoryType(category, mType string, excludeID uint) (bool, error) {
	var n int64
	q := r.db.Model(&model.MaintenanceStrategy{}).Where("category = ? AND type = ?", category, mType)
	if excludeID > 0 {
		q = q.Where("id <> ?", excludeID)
	}
	if err := q.Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// Count 统计策略总数（默认策略初始化幂等判断复用）。
func (r *MaintenanceStrategyRepository) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.MaintenanceStrategy{}).Count(&n).Error
	return n, err
}

// Save 更新策略。
func (r *MaintenanceStrategyRepository) Save(s *model.MaintenanceStrategy) error {
	return r.db.Save(s).Error
}
