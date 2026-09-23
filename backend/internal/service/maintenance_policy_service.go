package service

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/util"
	"gorm.io/gorm"
)

// MaintenancePolicyService 按设备类别配置的保养周期策略服务。
type MaintenancePolicyService struct {
	repo       *repository.MaintenancePolicyRepository
	deviceRepo *repository.DeviceRepository
	audit      *AuditService
	log        *slog.Logger
}

func NewMaintenancePolicyService(repo *repository.MaintenancePolicyRepository, deviceRepo *repository.DeviceRepository, audit *AuditService, log *slog.Logger) *MaintenancePolicyService {
	return &MaintenancePolicyService{repo: repo, deviceRepo: deviceRepo, audit: audit, log: log}
}

// List 返回策略列表：已配置策略 + 默认类别与设备台账中出现但尚未配置的类别（以默认周期补全展示）。
func (s *MaintenancePolicyService) List(operator string) ([]dto.PolicyListItem, error) {
	policies, err := s.repo.List()
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	merged := make(map[string]model.MaintenancePolicy)
	order := make([]string, 0)
	for i := range policies {
		if _, ok := merged[policies[i].Category]; !ok {
			order = append(order, policies[i].Category)
		}
		merged[policies[i].Category] = policies[i]
	}
	for _, c := range constants.DefaultDeviceCategories {
		if _, ok := merged[c]; !ok {
			merged[c] = s.defaultPolicy(c)
			order = append(order, c)
		}
	}
	devices, err := s.deviceRepo.ListAll()
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	for _, d := range devices {
		c := strings.TrimSpace(d.Category)
		if c == "" {
			continue
		}
		if _, ok := merged[c]; !ok {
			merged[c] = s.defaultPolicy(c)
			order = append(order, c)
		}
	}
	items := make([]dto.PolicyListItem, 0, len(order))
	for _, c := range order {
		p := merged[c]
		items = append(items, dto.ToPolicyListItem(&p))
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenancePolicyListed, operator))
	return items, nil
}

// Update 批量保存策略（按类别 upsert，重复提交幂等）。
func (s *MaintenancePolicyService) Update(req *dto.UpdatePolicyReq, operator string) ([]dto.PolicyListItem, error) {
	seen := make(map[string]bool, len(req.Items))
	records := make([]model.MaintenancePolicy, 0, len(req.Items))
	for _, item := range req.Items {
		category := strings.TrimSpace(item.Category)
		if category == "" {
			return nil, util.NewAppError(http.StatusBadRequest, constants.MsgPolicyCategoryEmpty, nil)
		}
		if seen[category] {
			return nil, util.NewAppError(http.StatusBadRequest, "设备类别重复: category="+category, nil)
		}
		seen[category] = true
		for _, v := range [4]int{item.DailyInterval, item.WeeklyInterval, item.MonthlyInterval, item.YearlyInterval} {
			if v < 0 || v > constants.MaxMaintenanceIntervalDays {
				return nil, util.NewAppError(http.StatusUnprocessableEntity, constants.MsgPolicyIntervalInvalid, nil)
			}
		}
		records = append(records, model.MaintenancePolicy{
			Category:        category,
			DailyInterval:   item.DailyInterval,
			WeeklyInterval:  item.WeeklyInterval,
			MonthlyInterval: item.MonthlyInterval,
			YearlyInterval:  item.YearlyInterval,
			UpdatedBy:       operator,
			UpdatedAt:       time.Now(),
		})
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		return s.repo.BatchUpsertTx(tx, records)
	}); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "保养周期策略保存失败", err)
	}
	for i := range records {
		r := records[i]
		s.log.Info(fmt.Sprintf(constants.LogMaintenancePolicyUpdated,
			r.Category, r.DailyInterval, r.WeeklyInterval, r.MonthlyInterval, r.YearlyInterval, operator))
		s.audit.Record(0, operator, "UPDATE", "maintenance_policy", r.Category,
			fmt.Sprintf("更新保养周期策略: category=%s 日检=%d 周检=%d 月检=%d 年检=%d",
				r.Category, r.DailyInterval, r.WeeklyInterval, r.MonthlyInterval, r.YearlyInterval), operator, "")
	}
	return s.List(operator)
}

// defaultPolicy 构造某类别的默认策略（未配置时的展示与生成基准）。
func (s *MaintenancePolicyService) defaultPolicy(category string) model.MaintenancePolicy {
	return model.MaintenancePolicy{
		Category:        category,
		DailyInterval:   constants.DefaultDailyInterval,
		WeeklyInterval:  constants.DefaultWeeklyInterval,
		MonthlyInterval: constants.DefaultMonthlyInterval,
		YearlyInterval:  constants.DefaultYearlyInterval,
	}
}

// EnsureDefaults 服务启动时为默认设备类别补齐策略（已存在的类别不覆盖）。
func (s *MaintenancePolicyService) EnsureDefaults() error {
	existing, err := s.repo.List()
	if err != nil {
		return fmt.Errorf("list maintenance policies: %w", err)
	}
	has := make(map[string]bool, len(existing))
	for _, p := range existing {
		has[p.Category] = true
	}
	now := time.Now()
	records := make([]model.MaintenancePolicy, 0, len(constants.DefaultDeviceCategories))
	for _, c := range constants.DefaultDeviceCategories {
		if has[c] {
			continue
		}
		p := s.defaultPolicy(c)
		p.CreatedAt = now
		p.UpdatedAt = now
		records = append(records, p)
	}
	if len(records) == 0 {
		return nil
	}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		return s.repo.BatchUpsertTx(tx, records)
	}); err != nil {
		return fmt.Errorf("seed default maintenance policies: %w", err)
	}
	s.log.Info(fmt.Sprintf(constants.LogMaintenancePolicySeeded, len(records)))
	return nil
}
