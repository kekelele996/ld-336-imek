package service

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/util"
)

// MaintenanceStrategyService 保养周期策略服务（按设备类别配置，可停用或调整）。
type MaintenanceStrategyService struct {
	repo  *repository.MaintenanceStrategyRepository
	audit *AuditService
	log   *slog.Logger
}

func NewMaintenanceStrategyService(repo *repository.MaintenanceStrategyRepository, audit *AuditService, log *slog.Logger) *MaintenanceStrategyService {
	return &MaintenanceStrategyService{repo: repo, audit: audit, log: log}
}

// List 查询周期策略（设备类别/启用状态过滤）。
func (s *MaintenanceStrategyService) List(category string, enabled *bool) ([]dto.MaintenanceStrategyItem, error) {
	list, err := s.repo.List(category, enabled)
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	return dto.ToMaintenanceStrategyList(list), nil
}

// Create 新建周期策略；同类别同类型唯一。
func (s *MaintenanceStrategyService) Create(req *dto.CreateMaintenanceStrategyReq, operator string) (*model.MaintenanceStrategy, error) {
	if !isPlanType(req.Type) {
		return nil, util.NewAppError(http.StatusBadRequest, constants.MsgStrategyTypeInvalid+": type="+req.Type, nil)
	}
	exists, err := s.repo.ExistsByCategoryType(req.Category, req.Type, 0)
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	if exists {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDuplicateStrategy+": category="+req.Category+" type="+req.Type, nil)
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	m := &model.MaintenanceStrategy{
		Category:     req.Category,
		Type:         req.Type,
		IntervalDays: req.IntervalDays,
		Enabled:      enabled,
		Remark:       req.Remark,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "创建保养周期策略失败: category="+req.Category, err)
	}
	s.log.Info(fmt.Sprintf(constants.LogStrategyCreated, m.ID, m.Category, m.Type, m.IntervalDays, m.Enabled))
	s.audit.Record(0, operator, "CREATE", "maintenance_strategy", util.Uint64String(m.ID),
		"创建保养周期策略: "+m.Category+"/"+util.MaintenanceTypeText(m.Type), operator, "")
	return m, nil
}

// Update 调整周期策略（周期天数调整/停用启用）。
func (s *MaintenanceStrategyService) Update(id uint, req *dto.UpdateMaintenanceStrategyReq, operator string) (*model.MaintenanceStrategy, error) {
	m, err := s.repo.FindByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(http.StatusNotFound, constants.MsgStrategyNotFound+": id="+util.Uint64String(id), nil)
	}
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	if req.IntervalDays != nil {
		m.IntervalDays = *req.IntervalDays
	}
	if req.Enabled != nil {
		m.Enabled = *req.Enabled
	}
	m.Remark = req.Remark
	if err := s.repo.Save(m); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "调整保养周期策略失败: id="+util.Uint64String(id)+" category="+m.Category, err)
	}
	s.log.Info(fmt.Sprintf(constants.LogStrategyUpdated, m.ID, m.Category, m.Type, m.IntervalDays, m.Enabled, operator))
	action := "UPDATE"
	if req.Enabled != nil && !*req.Enabled {
		action = "DISABLE"
	} else if req.Enabled != nil && *req.Enabled {
		action = "ENABLE"
	}
	s.audit.Record(0, operator, action, "maintenance_strategy", util.Uint64String(m.ID),
		"调整保养周期策略: "+m.Category+"/"+util.MaintenanceTypeText(m.Type), operator, "")
	return m, nil
}

// SeedDefaults 启动时为内置设备类别初始化默认周期策略（幂等：已有任何策略则跳过，停用状态不被覆盖）。
func (s *MaintenanceStrategyService) SeedDefaults() error {
	n, err := s.repo.Count()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	for _, category := range constants.DefaultDeviceCategories {
		for _, t := range constants.MaintenancePlanTypes {
			m := &model.MaintenanceStrategy{
				Category:     category,
				Type:         t,
				IntervalDays: constants.DefaultMaintenanceIntervalDays[t],
				Enabled:      true,
				Remark:       "系统默认策略",
			}
			if err := s.repo.Create(m); err != nil {
				return err
			}
		}
	}
	s.log.Info(fmt.Sprintf(constants.LogStrategySeeded, len(constants.DefaultDeviceCategories), len(constants.MaintenancePlanTypes)))
	return nil
}

// isPlanType 校验类型是否为可配置周期的保养类型（日检/周检/月检/年检）。
func isPlanType(t string) bool {
	for _, pt := range constants.MaintenancePlanTypes {
		if pt == t {
			return true
		}
	}
	return false
}
