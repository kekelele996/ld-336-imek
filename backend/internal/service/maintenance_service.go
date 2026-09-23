package service

import (
	"fmt"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/util"
	"gorm.io/gorm"
)

// MaintenanceService 维护保养与故障维修服务。
type MaintenanceService struct {
	repo     *repository.MaintenanceRepository
	device   *repository.DeviceRepository
	strategy *repository.MaintenanceStrategyRepository
	audit    *AuditService
	log      *slog.Logger
}

func NewMaintenanceService(repo *repository.MaintenanceRepository, device *repository.DeviceRepository, strategy *repository.MaintenanceStrategyRepository, audit *AuditService, log *slog.Logger) *MaintenanceService {
	return &MaintenanceService{repo: repo, device: device, strategy: strategy, audit: audit, log: log}
}

// Create 创建保养/维修工单（报修或计划执行）。
func (s *MaintenanceService) Create(req *dto.CreateMaintenanceReq, operator string) (*model.MaintenanceRecord, error) {
	d, err := s.device.FindByID(req.DeviceID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(http.StatusNotFound, "设备不存在: device_id="+util.Uint64String(req.DeviceID), nil)
	}
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	if d.Status == constants.DeviceStatusScrapped {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDeviceInScrapped, nil)
	}
	m := &model.MaintenanceRecord{
		RecordNo:         util.GenSerial("MT"),
		DeviceID:         d.ID,
		DeviceName:       d.Name,
		Type:             req.Type,
		Status:           constants.MaintenanceStatusPending,
		PlannedDate:      req.PlannedDate,
		Content:          req.Content,
		FaultDescription: req.FaultDescription,
		Engineer:         req.Engineer,
		CreatedBy:        operator,
	}
	if err := s.repo.Create(m); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "创建工单失败: device_name="+d.Name, err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceCreated, m.RecordNo, m.DeviceID, m.Type, m.Status))
	s.audit.Record(0, operator, "CREATE", "maintenance", util.Uint64String(m.ID), "创建保养/维修工单: "+m.RecordNo, operator, "")
	return m, nil
}

// GeneratePlans 按设备类别周期策略自动生成保养计划：
// 仅处理启用策略；设备存在同类待处理/处理中工单则跳过；首次排到次日，
// 之后从上次完成日顺延周期；逾期计划保留原日期（不向前补齐到今日）。
// 每台设备独立事务并锁定设备行，连续点击或两人同时操作也不会产生重复计划。
func (s *MaintenanceService) GeneratePlans(operator string) (int, error) {
	strategies, err := s.strategy.ListEnabled()
	if err != nil {
		return 0, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	byCategory := make(map[string][]model.MaintenanceStrategy, len(strategies))
	for _, st := range strategies {
		byCategory[st.Category] = append(byCategory[st.Category], st)
	}

	devices, _, err := s.device.List(1, 100000, "", "", "", "")
	if err != nil {
		return 0, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	created := 0
	now := time.Now()
	for _, d := range devices {
		if d.Status == constants.DeviceStatusScrapped {
			continue
		}
		strategiesForDevice := byCategory[d.Category]
		if len(strategiesForDevice) == 0 {
			continue
		}
		n, err := s.generateForDevice(d, strategiesForDevice, now, operator)
		if err != nil {
			return created, err
		}
		created += n
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenancePlansGenerated, created, operator))
	return created, nil
}

// generateForDevice 单台设备的计划生成（独立事务 + 设备行锁，并发安全）。
func (s *MaintenanceService) generateForDevice(d model.Device, strategies []model.MaintenanceStrategy, now time.Time, operator string) (int, error) {
	created := 0
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		// 锁定设备行：两个并发生成请求在此串行化，后执行者能看到前者插入的待处理工单。
		locked, err := s.device.FindByIDForUpdate(tx, d.ID)
		if err != nil {
			return err
		}
		if locked.Status == constants.DeviceStatusScrapped {
			return nil
		}
		for _, st := range strategies {
			n, err := s.repo.CountActiveTx(tx, d.ID, st.Type, activeMaintenanceStatuses())
			if err != nil {
				return err
			}
			if n > 0 {
				// 已有同类待处理/处理中工单（含逾期未完成计划），跳过且保留原日期。
				continue
			}
			planned := nextPlanDate(tx, s.repo, d.ID, st.Type, st.IntervalDays, now)
			m := &model.MaintenanceRecord{
				RecordNo:    util.GenSerial("MT"),
				DeviceID:    locked.ID,
				DeviceName:  locked.Name,
				Type:        st.Type,
				Status:      constants.MaintenanceStatusPending,
				PlannedDate: &planned,
				Content:     "自动生成" + util.MaintenanceTypeText(st.Type) + "保养计划",
				CreatedBy:   operator,
			}
			if err := tx.Create(m).Error; err != nil {
				return err
			}
			created++
		}
		return nil
	})
	if err != nil {
		return 0, wrapSvcErr(err)
	}
	return created, nil
}

// nextPlanDate 计算下一期计划日期：有完成记录从上次完成日顺延周期，否则首次排到次日；
// 顺延结果即使已逾期也保留原日期（不补齐到今日）。
func nextPlanDate(tx *gorm.DB, repo *repository.MaintenanceRepository, deviceID uint, mType string, intervalDays int, now time.Time) time.Time {
	base := startOfDay(now).AddDate(0, 0, 1) // 初次排到次日 00:00
	last, err := repo.LastCompletedTx(tx, deviceID, mType, constants.MaintenanceStatusCompleted)
	if err == nil && last.ExecutedDate != nil {
		base = startOfDay(*last.ExecutedDate).AddDate(0, 0, intervalDays)
	}
	return base
}

// activeMaintenanceStatuses 生成计划时视为占用的状态：待处理 + 处理中。
func activeMaintenanceStatuses() []string {
	return []string{constants.MaintenanceStatusPending, constants.MaintenanceStatusInProgress}
}

// startOfDay 截断到当日零点，保证周期顺延按自然日计算。
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// chainNextPlan 本期完成后在同一事务内顺延生成下一期计划（仅周期保养类型，且策略处于启用状态）。
// 调用前必须已锁定设备行，保证与并发生成请求互斥。
func (s *MaintenanceService) chainNextPlan(tx *gorm.DB, completed *model.MaintenanceRecord, category string, now time.Time, operator string) error {
	if completed.Type == constants.MaintenanceTypeRepair {
		return nil
	}
	st, err := s.strategy.FindByCategoryTypeTx(tx, category, completed.Type)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // 未配置策略的类别类型不顺延。
		}
		return err
	}
	if !st.Enabled {
		return nil // 策略已停用不顺延。
	}
	n, err := s.repo.CountActiveTx(tx, completed.DeviceID, completed.Type, activeMaintenanceStatuses())
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	planned := startOfDay(now).AddDate(0, 0, st.IntervalDays)
	m := &model.MaintenanceRecord{
		RecordNo:    util.GenSerial("MT"),
		DeviceID:    completed.DeviceID,
		DeviceName:  completed.DeviceName,
		Type:        completed.Type,
		Status:      constants.MaintenanceStatusPending,
		PlannedDate: &planned,
		Content:     "完成上一期后自动顺延的" + util.MaintenanceTypeText(completed.Type) + "保养计划",
		CreatedBy:   operator,
	}
	if err := tx.Create(m).Error; err != nil {
		return err
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceChainCreated, completed.RecordNo, completed.DeviceID, completed.Type, util.FormatDate(&planned)))
	return nil
}

// List 分页查询保养/维修记录。
func (s *MaintenanceService) List(page, pageSize int, deviceID uint, mType, status string) (*util.PageResult, error) {
	list, total, err := s.repo.List(page, pageSize, deviceID, mType, status)
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	return &util.PageResult{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Start 开始执行工单。
func (s *MaintenanceService) Start(id uint, req *dto.StartMaintenanceReq, operator string) (*model.MaintenanceRecord, error) {
	var updated *model.MaintenanceRecord
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		m, err := s.repo.FindByIDForUpdate(tx, id)
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(http.StatusNotFound, "工单不存在: id="+util.Uint64String(id), nil)
		}
		if err != nil {
			return err
		}
		if m.Status != constants.MaintenanceStatusPending {
			return util.NewAppError(http.StatusConflict, constants.MsgInvalidStatus, nil)
		}
		m.Status = constants.MaintenanceStatusInProgress
		m.Engineer = req.Engineer
		if err := s.repo.UpdateTx(tx, m); err != nil {
			return err
		}
		// 维修类工单开始时设备进入维修中状态。
		if m.Type == constants.MaintenanceTypeRepair {
			if err := s.device.UpdateStatusTx(tx, m.DeviceID, constants.DeviceStatusUnderMaintenance); err != nil {
				return err
			}
		}
		updated = m
		return nil
	})
	if err != nil {
		return nil, wrapSvcErr(err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceStarted, updated.RecordNo, updated.Engineer, updated.Status))
	s.audit.Record(0, operator, "START", "maintenance", util.Uint64String(updated.ID), "开始执行: "+updated.RecordNo, operator, "")
	return updated, nil
}

// Complete 完成工单（更新工时/成本/配件，恢复设备状态；周期保养完成后顺延生成下一期计划）。
func (s *MaintenanceService) Complete(id uint, req *dto.CompleteMaintenanceReq, operator string) (*model.MaintenanceRecord, error) {
	var updated *model.MaintenanceRecord
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		m, err := s.repo.FindByIDForUpdate(tx, id)
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(http.StatusNotFound, "工单不存在: id="+util.Uint64String(id), nil)
		}
		if err != nil {
			return err
		}
		if m.Status != constants.MaintenanceStatusInProgress {
			return util.NewAppError(http.StatusConflict, constants.MsgInvalidStatus, nil)
		}
		now := time.Now()
		m.Status = constants.MaintenanceStatusCompleted
		m.ExecutedDate = &now
		m.Content = req.Content
		m.ReplacedParts = req.ReplacedParts
		m.WorkHours = req.WorkHours
		m.Cost = req.Cost
		m.RepairResult = req.RepairResult
		if err := s.repo.UpdateTx(tx, m); err != nil {
			return err
		}
		// 维修完成后设备恢复使用中。
		if m.Type == constants.MaintenanceTypeRepair {
			if err := s.device.UpdateStatusTx(tx, m.DeviceID, constants.DeviceStatusInUse); err != nil {
				return err
			}
		}
		if err := tx.Model(&model.Device{}).Where("id = ?", m.DeviceID).Update("last_maintenance_at", now).Error; err != nil {
			return err
		}
		// 锁定设备行后顺延下一期：与并发的生成计划请求互斥，保证同一设备不产生重复计划。
		locked, err := s.device.FindByIDForUpdate(tx, m.DeviceID)
		if err != nil {
			return err
		}
		if locked.Status != constants.DeviceStatusScrapped {
			if err := s.chainNextPlan(tx, m, locked.Category, now, operator); err != nil {
				return err
			}
		}
		updated = m
		return nil
	})
	if err != nil {
		return nil, wrapSvcErr(err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceCompleted, updated.RecordNo, updated.DeviceID, updated.Cost, updated.Status))
	s.audit.Record(0, operator, "COMPLETE", "maintenance", util.Uint64String(updated.ID), "完成工单: "+updated.RecordNo, operator, "")
	return updated, nil
}

// Cancel 取消工单。
func (s *MaintenanceService) Cancel(id uint, req *dto.CancelMaintenanceReq, operator string) (*model.MaintenanceRecord, error) {
	var updated *model.MaintenanceRecord
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		m, err := s.repo.FindByIDForUpdate(tx, id)
		if errors.Is(err, repository.ErrNotFound) {
			return util.NewAppError(http.StatusNotFound, "工单不存在: id="+util.Uint64String(id), nil)
		}
		if err != nil {
			return err
		}
		if m.Status != constants.MaintenanceStatusPending {
			return util.NewAppError(http.StatusConflict, constants.MsgInvalidStatus, nil)
		}
		m.Status = constants.MaintenanceStatusCancelled
		if req.Reason != "" {
			m.RepairResult = "取消原因: " + req.Reason
		}
		if err := s.repo.UpdateTx(tx, m); err != nil {
			return err
		}
		updated = m
		return nil
	})
	if err != nil {
		return nil, wrapSvcErr(err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceCancelled, updated.RecordNo, req.Reason, updated.Status))
	s.audit.Record(0, operator, "CANCEL", "maintenance", util.Uint64String(updated.ID), "取消工单: "+updated.RecordNo, operator, "")
	return updated, nil
}
