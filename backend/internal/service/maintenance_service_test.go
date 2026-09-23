package service

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
)

// newMaintenanceSvc 构建带策略仓储的保养服务测试实例。
func newMaintenanceSvc(env *testEnv) *MaintenanceService {
	return NewMaintenanceService(
		repository.NewMaintenanceRepository(env.db),
		repository.NewDeviceRepository(env.db),
		repository.NewMaintenanceStrategyRepository(env.db),
		env.audit, env.logger,
	)
}

func seedDevice(t *testing.T, env *testEnv, assetCode, category, status string) model.Device {
	d := model.Device{AssetCode: assetCode, Name: "设备-" + assetCode, Category: category, Department: "ICU", Status: status}
	if err := env.db.Create(&d).Error; err != nil {
		t.Fatalf("seed device failed: %v", err)
	}
	return d
}

func seedStrategy(t *testing.T, env *testEnv, category, mType string, interval int, enabled bool) model.MaintenanceStrategy {
	st := model.MaintenanceStrategy{Category: category, Type: mType, IntervalDays: interval, Enabled: enabled}
	if err := env.db.Create(&st).Error; err != nil {
		t.Fatalf("seed strategy failed: %v", err)
	}
	return st
}

func listRecords(t *testing.T, env *testEnv, deviceID uint, mType, status string) []model.MaintenanceRecord {
	var list []model.MaintenanceRecord
	q := env.db.Where("device_id = ?", deviceID)
	if mType != "" {
		q = q.Where("type = ?", mType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&list).Error; err != nil {
		t.Fatalf("list records failed: %v", err)
	}
	return list
}

// TestGeneratePlansFirstTimeNextDay 初次生成排到次日 00:00。
func TestGeneratePlansFirstTimeNextDay(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-1", "生命支持", constants.DeviceStatusInUse)
	seedStrategy(t, env, "生命支持", constants.MaintenanceTypeDaily, 1, true)

	n, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("created = %d, want 1", n)
	}
	records := listRecords(t, env, d.ID, constants.MaintenanceTypeDaily, constants.MaintenanceStatusPending)
	if len(records) != 1 {
		t.Fatalf("pending daily records = %d, want 1", len(records))
	}
	now := time.Now()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)
	if !records[0].PlannedDate.Equal(want) {
		t.Errorf("planned_date = %v, want 次日 %v", records[0].PlannedDate, want)
	}
}

// TestGeneratePlansOnlyEnabledStrategies 停用策略不参与生成。
func TestGeneratePlansOnlyEnabledStrategies(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-2", "影像设备", constants.DeviceStatusInUse)
	seedStrategy(t, env, "影像设备", constants.MaintenanceTypeDaily, 1, true)
	seedStrategy(t, env, "影像设备", constants.MaintenanceTypeWeekly, 7, false) // 已停用

	n, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("created = %d, want 1（停用策略不生成）", n)
	}
	if got := len(listRecords(t, env, d.ID, constants.MaintenanceTypeWeekly, "")); got != 0 {
		t.Errorf("weekly records = %d, want 0", got)
	}
}

// TestGeneratePlansSkipsActiveOrders 有同类待处理/处理中工单时跳过。
func TestGeneratePlansSkipsActiveOrders(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-3", "检验设备", constants.DeviceStatusInUse)
	seedStrategy(t, env, "检验设备", constants.MaintenanceTypeDaily, 1, true)
	seedStrategy(t, env, "检验设备", constants.MaintenanceTypeWeekly, 7, true)

	// 预置一条待处理日检与一条处理中周检。
	pending := model.MaintenanceRecord{RecordNo: "MT-P1", DeviceID: d.ID, DeviceName: d.Name, Type: constants.MaintenanceTypeDaily, Status: constants.MaintenanceStatusPending}
	inProgress := model.MaintenanceRecord{RecordNo: "MT-P2", DeviceID: d.ID, DeviceName: d.Name, Type: constants.MaintenanceTypeWeekly, Status: constants.MaintenanceStatusInProgress}
	if err := env.db.Create(&pending).Error; err != nil {
		t.Fatalf("seed pending failed: %v", err)
	}
	if err := env.db.Create(&inProgress).Error; err != nil {
		t.Fatalf("seed in_progress failed: %v", err)
	}

	n, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if n != 0 {
		t.Errorf("created = %d, want 0（待处理/处理中工单应跳过）", n)
	}
	if got := len(listRecords(t, env, d.ID, "", "")); got != 2 {
		t.Errorf("total records = %d, want 2（不产生重复计划）", got)
	}
}

// TestGeneratePlansIdempotent 连续点击生成不产生重复计划。
func TestGeneratePlansIdempotent(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-4", "手术器械", constants.DeviceStatusInUse)
	seedStrategy(t, env, "手术器械", constants.MaintenanceTypeMonthly, 30, true)

	for i := 0; i < 3; i++ {
		if _, err := svc.GeneratePlans("admin"); err != nil {
			t.Fatalf("GeneratePlans round %d failed: %v", i, err)
		}
	}
	if got := len(listRecords(t, env, d.ID, constants.MaintenanceTypeMonthly, constants.MaintenanceStatusPending)); got != 1 {
		t.Errorf("pending monthly = %d, want 1（连续生成不重复）", got)
	}
}

// TestGeneratePlansFromLastCompleted 之后从上次完成日顺延；逾期保留原日期。
func TestGeneratePlansFromLastCompleted(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-5", "消毒设备", constants.DeviceStatusInUse)
	seedStrategy(t, env, "消毒设备", constants.MaintenanceTypeMonthly, 30, true)

	// 上次完成于 40 天前 → 顺延 30 天后计划日期已逾期 10 天，仍保留原日期。
	executed := time.Now().AddDate(0, 0, -40)
	done := model.MaintenanceRecord{RecordNo: "MT-C1", DeviceID: d.ID, DeviceName: d.Name, Type: constants.MaintenanceTypeMonthly, Status: constants.MaintenanceStatusCompleted, ExecutedDate: &executed}
	if err := env.db.Create(&done).Error; err != nil {
		t.Fatalf("seed completed failed: %v", err)
	}

	n, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("created = %d, want 1", n)
	}
	records := listRecords(t, env, d.ID, constants.MaintenanceTypeMonthly, constants.MaintenanceStatusPending)
	if len(records) != 1 {
		t.Fatalf("pending records = %d, want 1", len(records))
	}
	want := time.Date(executed.Year(), executed.Month(), executed.Day(), 0, 0, 0, 0, executed.Location()).AddDate(0, 0, 30)
	if !records[0].PlannedDate.Equal(want) {
		t.Errorf("planned_date = %v, want 上次完成日顺延 %v（逾期保留原日期）", records[0].PlannedDate, want)
	}
	if !records[0].PlannedDate.Before(time.Now()) {
		t.Errorf("expected overdue planned_date in the past, got %v", records[0].PlannedDate)
	}
}

// TestGeneratePlansSkipsScrappedAndUnknownCategory 报废设备与无策略类别不生成。
func TestGeneratePlansSkipsScrappedAndUnknownCategory(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	seedDevice(t, env, "MA-6", "生命支持", constants.DeviceStatusScrapped)
	seedDevice(t, env, "MA-7", "未配置类别", constants.DeviceStatusInUse)
	seedStrategy(t, env, "生命支持", constants.MaintenanceTypeDaily, 1, true)

	n, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if n != 0 {
		t.Errorf("created = %d, want 0", n)
	}
}

// TestCompleteChainsNextPeriod 本期完成后自动顺延下一期，且生成接口不会重复补单。
func TestCompleteChainsNextPeriod(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-8", "康复设备", constants.DeviceStatusInUse)
	seedStrategy(t, env, "康复设备", constants.MaintenanceTypeMonthly, 30, true)

	m, err := svc.Create(&dto.CreateMaintenanceReq{DeviceID: d.ID, Type: constants.MaintenanceTypeMonthly}, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := svc.Start(m.ID, &dto.StartMaintenanceReq{Engineer: "王工"}, "admin"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := svc.Complete(m.ID, &dto.CompleteMaintenanceReq{Content: "已保养"}, "admin"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	pending := listRecords(t, env, d.ID, constants.MaintenanceTypeMonthly, constants.MaintenanceStatusPending)
	if len(pending) != 1 {
		t.Fatalf("chained pending = %d, want 1（完成后顺延下一期）", len(pending))
	}
	now := time.Now()
	want := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 30)
	if !pending[0].PlannedDate.Equal(want) {
		t.Errorf("chained planned_date = %v, want 完成日顺延30天 %v", pending[0].PlannedDate, want)
	}

	// 再点生成：已有待处理工单，不能重复。
	created, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans failed: %v", err)
	}
	if created != 0 {
		t.Errorf("created = %d, want 0（顺延计划已存在）", created)
	}
}

// TestCompleteRepairDoesNotChain 故障维修完成不顺延周期计划。
func TestCompleteRepairDoesNotChain(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-9", "生命支持", constants.DeviceStatusInUse)
	seedStrategy(t, env, "生命支持", constants.MaintenanceTypeDaily, 1, true)

	m, err := svc.Create(&dto.CreateMaintenanceReq{DeviceID: d.ID, Type: constants.MaintenanceTypeRepair}, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := svc.Start(m.ID, &dto.StartMaintenanceReq{Engineer: "王工"}, "admin"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := svc.Complete(m.ID, &dto.CompleteMaintenanceReq{Content: "已修复"}, "admin"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if got := len(listRecords(t, env, d.ID, "", "")); got != 1 {
		t.Errorf("records = %d, want 1（维修完成不生成保养计划）", got)
	}
}

// TestCompleteDisabledStrategyDoesNotChain 策略停用后完成不顺延。
func TestCompleteDisabledStrategyDoesNotChain(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newMaintenanceSvc(env)
	d := seedDevice(t, env, "MA-10", "生命支持", constants.DeviceStatusInUse)
	seedStrategy(t, env, "生命支持", constants.MaintenanceTypeDaily, 1, false)

	m, err := svc.Create(&dto.CreateMaintenanceReq{DeviceID: d.ID, Type: constants.MaintenanceTypeDaily}, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := svc.Start(m.ID, &dto.StartMaintenanceReq{Engineer: "王工"}, "admin"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := svc.Complete(m.ID, &dto.CompleteMaintenanceReq{Content: "已检"}, "admin"); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if got := len(listRecords(t, env, d.ID, constants.MaintenanceTypeDaily, constants.MaintenanceStatusPending)); got != 0 {
		t.Errorf("pending daily = %d, want 0（策略停用不顺延）", got)
	}
}
