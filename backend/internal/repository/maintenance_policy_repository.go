package repository

import (
	"errors"

	"github.com/medasset/medasset/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// MaintenancePolicyRepository 按设备类别配置的保养周期策略仓储。
type MaintenancePolicyRepository struct {
	db *gorm.DB
}

func NewMaintenancePolicyRepository(db *gorm.DB) *MaintenancePolicyRepository {
	return &MaintenancePolicyRepository{db: db}
}

// DB 返回底层数据库句柄（供 service 层开启事务）。
func (r *MaintenancePolicyRepository) DB() *gorm.DB { return r.db }

// List 查询全部周期策略。
func (r *MaintenancePolicyRepository) List() ([]model.MaintenancePolicy, error) {
	var list []model.MaintenancePolicy
	err := r.db.Order("id ASC").Find(&list).Error
	return list, err
}

// MapByCategory 以设备类别为键返回策略映射。
func (r *MaintenancePolicyRepository) MapByCategory() (map[string]model.MaintenancePolicy, error) {
	list, err := r.List()
	if err != nil {
		return nil, err
	}
	out := make(map[string]model.MaintenancePolicy, len(list))
	for i := range list {
		out[list[i].Category] = list[i]
	}
	return out, nil
}

// FindByCategory 按设备类别查询策略。
func (r *MaintenancePolicyRepository) FindByCategory(category string) (*model.MaintenancePolicy, error) {
	var p model.MaintenancePolicy
	err := r.db.Where("category = ?", category).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &p, err
}

// UpsertTx 在事务内按类别新增或更新策略（同类别重复提交幂等）。
func (r *MaintenancePolicyRepository) UpsertTx(tx *gorm.DB, p *model.MaintenancePolicy) error {
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "category"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"daily_interval", "weekly_interval", "monthly_interval", "yearly_interval",
			"updated_by", "updated_at",
		}),
	}).Create(p).Error
}

// BatchUpsertTx 在事务内批量新增或更新策略。
func (r *MaintenancePolicyRepository) BatchUpsertTx(tx *gorm.DB, items []model.MaintenancePolicy) error {
	if len(items) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "category"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"daily_interval", "weekly_interval", "monthly_interval", "yearly_interval",
			"updated_by", "updated_at",
		}),
	}).Create(&items).Error
}
