package service

import (
	"errors"
	"fmt"
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
	repo   *repository.MaintenanceRepository
	device *repository.DeviceRepository
	policy *repository.MaintenancePolicyRepository
	audit  *AuditService
	log    *slog.Logger
}

func NewMaintenanceService(repo *repository.MaintenanceRepository, device *repository.DeviceRepository, policy *repository.MaintenancePolicyRepository, audit *AuditService, log *slog.Logger) *MaintenanceService {
	return &MaintenanceService{repo: repo, device: device, policy: policy, audit: audit, log: log}
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

// GeneratePlansResult 批量生成结果。
type GeneratePlansResult struct {
	Created int `json:"created"`
	Skipped int `json:"skipped"`
}

// GeneratePlans 按设备类别周期策略批量生成保养计划。
// 规则：
//   - 仅处理已启用（间隔 > 0）的策略，间隔为 0 表示停用；
//   - 每台设备独立事务并先锁定设备行，同类存在待处理/处理中工单则跳过，保证连续点击或两人并发不产生重复计划；
//   - 首次计划排在次日；之后以上次完成日期 + 周期顺延，计算结果即使已逾期也保留原日期不顺延。
func (s *MaintenanceService) GeneratePlans(operator string) (*GeneratePlansResult, error) {
	devices, err := s.device.ListAll()
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	policies, err := s.policy.MapByCategory()
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	result := &GeneratePlansResult{}
	now := truncateToDate(time.Now())
	for i := range devices {
		d := devices[i]
		if d.Status == constants.DeviceStatusScrapped {
			continue
		}
		p, ok := policies[d.Category]
		if !ok {
			// 该类别尚未配置策略：使用内置默认周期（与策略页展示一致）。
			p = defaultPolicyFor(d.Category)
		}
		planTypes := enabledPlanTypes(p)
		if len(planTypes) == 0 {
			continue
		}
		if err := s.generateForDevice(&d, p, planTypes, now, operator, result); err != nil {
			// 单台设备失败不影响其余设备。
			s.log.Error("生成设备保养计划失败", "device_id", d.ID, "err", err)
		}
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenancePlansGenerated, result.Created, result.Skipped, operator))
	s.audit.Record(0, operator, "GENERATE", "maintenance", "0",
		fmt.Sprintf("批量生成保养计划: created=%d skipped=%d", result.Created, result.Skipped), operator, "")
	return result, nil
}

// generateForDevice 在单设备事务内完成查重与建单（设备行锁串行化同设备并发）。
func (s *MaintenanceService) generateForDevice(d *model.Device, p model.MaintenancePolicy, planTypes []planTypeInterval, now time.Time, operator string, result *GeneratePlansResult) error {
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		locked, err := s.device.FindByIDForUpdate(tx, d.ID)
		if err != nil {
			return err
		}
		if locked.Status == constants.DeviceStatusScrapped {
			return nil
		}
		for _, pt := range planTypes {
			open, err := s.repo.CountOpenByTypeTx(tx, d.ID, pt.mType)
			if err != nil {
				return err
			}
			if open > 0 {
				result.Skipped++
				continue
			}
			planned := s.calcPlannedDate(tx, d.ID, pt.mType, pt.interval, now)
			m := &model.MaintenanceRecord{
				RecordNo:    util.GenSerial("MT"),
				DeviceID:    d.ID,
				DeviceName:  d.Name,
				Type:        pt.mType,
				Status:      constants.MaintenanceStatusPending,
				PlannedDate: &planned,
				Content:     "自动生成" + util.MaintenanceTypeText(pt.mType) + "保养计划",
				CreatedBy:   operator,
			}
			if err := s.repo.CreateTx(tx, m); err != nil {
				return err
			}
			result.Created++
		}
		return nil
	})
}

type planTypeInterval struct {
	mType    string
	interval int
}

// enabledPlanTypes 返回策略中已启用（间隔 > 0）的保养类型。
func enabledPlanTypes(p model.MaintenancePolicy) []planTypeInterval {
	out := make([]planTypeInterval, 0, 4)
	if p.DailyInterval > 0 {
		out = append(out, planTypeInterval{constants.MaintenanceTypeDaily, p.DailyInterval})
	}
	if p.WeeklyInterval > 0 {
		out = append(out, planTypeInterval{constants.MaintenanceTypeWeekly, p.WeeklyInterval})
	}
	if p.MonthlyInterval > 0 {
		out = append(out, planTypeInterval{constants.MaintenanceTypeMonthly, p.MonthlyInterval})
	}
	if p.YearlyInterval > 0 {
		out = append(out, planTypeInterval{constants.MaintenanceTypeYearly, p.YearlyInterval})
	}
	return out
}

// defaultPolicyFor 未配置策略的类别使用内置默认周期。
func defaultPolicyFor(category string) model.MaintenancePolicy {
	return model.MaintenancePolicy{
		Category:        category,
		DailyInterval:   constants.DefaultDailyInterval,
		WeeklyInterval:  constants.DefaultWeeklyInterval,
		MonthlyInterval: constants.DefaultMonthlyInterval,
		YearlyInterval:  constants.DefaultYearlyInterval,
	}
}

// calcPlannedDate 计算计划日期：
//   - 该设备该类型无已完成工单（首次）：次日；
//   - 之后：上次执行（完成）日期 + 周期天数；逾期也保留该日期，不前推到今天/明天。
//
// 日期统一截断到零点。
func (s *MaintenanceService) calcPlannedDate(tx *gorm.DB, deviceID uint, mType string, interval int, now time.Time) time.Time {
	last, err := s.repo.LatestCompletedByTypeTx(tx, deviceID, mType)
	if err != nil || last == nil || last.ExecutedDate == nil {
		return truncateToDate(now.AddDate(0, 0, 1))
	}
	return truncateToDate(last.ExecutedDate.AddDate(0, 0, interval))
}

// truncateToDate 截断到当天零点（本地时区）。
func truncateToDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
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

// Complete 完成工单（更新工时/成本/配件，恢复设备状态），完成后自动顺延生成下一期保养计划。
func (s *MaintenanceService) Complete(id uint, req *dto.CompleteMaintenanceReq, operator string) (*model.MaintenanceRecord, error) {
	var updated *model.MaintenanceRecord
	var nextPlanned *time.Time
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
		updated = m

		// 本期完成后下一期接着生成：按类别策略顺延同类保养计划（锁定设备行，有未完结工单则不重复生成）。
		if interval, ok := s.intervalForTx(tx, m.DeviceID, m.Type); ok {
			d, err := s.device.FindByIDForUpdate(tx, m.DeviceID)
			if err != nil {
				return err
			}
			open, err := s.repo.CountOpenByTypeTx(tx, m.DeviceID, m.Type)
			if err != nil {
				return err
			}
			if open == 0 && d.Status != constants.DeviceStatusScrapped {
				planned := s.calcPlannedDate(tx, m.DeviceID, m.Type, interval, truncateToDate(now))
				next := &model.MaintenanceRecord{
					RecordNo:    util.GenSerial("MT"),
					DeviceID:    m.DeviceID,
					DeviceName:  m.DeviceName,
					Type:        m.Type,
					Status:      constants.MaintenanceStatusPending,
					PlannedDate: &planned,
					Content:     "完成上一期后自动顺延生成" + util.MaintenanceTypeText(m.Type) + "保养计划",
					CreatedBy:   operator,
				}
				if err := s.repo.CreateTx(tx, next); err != nil {
					return err
				}
				nextPlanned = &planned
			}
		}
		return nil
	})
	if err != nil {
		return nil, wrapSvcErr(err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenanceCompleted, updated.RecordNo, updated.DeviceID, updated.Cost, updated.Status))
	if nextPlanned != nil {
		s.log.Info(fmt.Sprintf(constants.LogMaintenanceNextPlan, updated.RecordNo, updated.DeviceID, updated.Type, util.FormatDate(nextPlanned)))
	}
	s.audit.Record(0, operator, "COMPLETE", "maintenance", util.Uint64String(updated.ID), "完成工单: "+updated.RecordNo, operator, "")
	return updated, nil
}

// intervalForTx 在事务内查询某设备某保养类型在类别策略中的周期；停用或维修类返回 false。
func (s *MaintenanceService) intervalForTx(tx *gorm.DB, deviceID uint, mType string) (int, bool) {
	var d model.Device
	if err := tx.First(&d, deviceID).Error; err != nil {
		return 0, false
	}
	var p model.MaintenancePolicy
	err := tx.Where("category = ?", d.Category).First(&p).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		p = defaultPolicyFor(d.Category)
	} else if err != nil {
		return 0, false
	}
	switch mType {
	case constants.MaintenanceTypeDaily:
		return p.DailyInterval, p.DailyInterval > 0
	case constants.MaintenanceTypeWeekly:
		return p.WeeklyInterval, p.WeeklyInterval > 0
	case constants.MaintenanceTypeMonthly:
		return p.MonthlyInterval, p.MonthlyInterval > 0
	case constants.MaintenanceTypeYearly:
		return p.YearlyInterval, p.YearlyInterval > 0
	default:
		return 0, false
	}
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
