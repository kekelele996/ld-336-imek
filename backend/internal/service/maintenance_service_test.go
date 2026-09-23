package service

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
)

func newMaintenanceTestEnv(t *testing.T) (*MaintenanceService, *repository.DeviceRepository, *repository.MaintenanceRepository, *repository.MaintenancePolicyRepository) {
	env := newTestServiceEnv(t)
	deviceRepo := repository.NewDeviceRepository(env.db)
	mRepo := repository.NewMaintenanceRepository(env.db)
	pRepo := repository.NewMaintenancePolicyRepository(env.db)
	svc := NewMaintenanceService(mRepo, deviceRepo, pRepo, env.audit, env.logger)
	return svc, deviceRepo, mRepo, pRepo
}

func TestGeneratePlans_FirstScheduledTomorrow(t *testing.T) {
	svc, deviceRepo, mRepo, pRepo := newMaintenanceTestEnv(t)
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "影像设备", DailyInterval: 1, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365,
	})
	d := &model.Device{AssetCode: "MA-FIRST", Name: "CT", Category: "影像设备", Status: constants.DeviceStatusInUse}
	if err := deviceRepo.Create(d); err != nil {
		t.Fatal(err)
	}

	res, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatalf("GeneratePlans: %v", err)
	}
	if res.Created != 4 || res.Skipped != 0 {
		t.Fatalf("created=%d skipped=%d, want 4/0", res.Created, res.Skipped)
	}
	list, _, _ := mRepo.List(1, 50, d.ID, "", "")
	tomorrow := truncateToDate(time.Now().AddDate(0, 0, 1))
	if len(list) != 4 {
		t.Fatalf("want 4 records, got %d", len(list))
	}
	for _, m := range list {
		if m.PlannedDate == nil || !sameDate(*m.PlannedDate, tomorrow) {
			t.Errorf("type=%s planned=%v, want tomorrow %s", m.Type, m.PlannedDate, tomorrow)
		}
	}
}

func TestGeneratePlans_SecondRunSkipsOpen(t *testing.T) {
	svc, deviceRepo, _, pRepo := newMaintenanceTestEnv(t)
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "生命支持", DailyInterval: 1, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365,
	})
	d := &model.Device{AssetCode: "MA-SKIP", Name: "监护仪", Category: "生命支持", Status: constants.DeviceStatusInUse}
	deviceRepo.Create(d)

	first, err := svc.GeneratePlans("admin")
	if err != nil || first.Created != 4 {
		t.Fatalf("first run: %+v err=%v", first, err)
	}
	second, err := svc.GeneratePlans("admin")
	if err != nil || second.Created != 0 || second.Skipped != 4 {
		t.Fatalf("second run: %+v err=%v, want created=0 skipped=4", second, err)
	}
}

func TestGeneratePlans_DisabledTypeSkipped(t *testing.T) {
	svc, deviceRepo, mRepo, pRepo := newMaintenanceTestEnv(t)
	// 日检停用（0），其余启用。
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "检验设备", DailyInterval: 0, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365,
	})
	d := &model.Device{AssetCode: "MA-DIS", Name: "分析仪", Category: "检验设备", Status: constants.DeviceStatusInUse}
	deviceRepo.Create(d)

	res, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 3 {
		t.Fatalf("created=%d, want 3 (daily disabled)", res.Created)
	}
	list, _, _ := mRepo.List(1, 50, d.ID, constants.MaintenanceTypeDaily, "")
	if len(list) != 0 {
		t.Fatalf("daily plan should not be generated for disabled policy, got %d", len(list))
	}
}

func TestGeneratePlans_RolloverFromLastCompletion(t *testing.T) {
	svc, deviceRepo, mRepo, pRepo := newMaintenanceTestEnv(t)
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "影像设备", DailyInterval: 1, WeeklyInterval: 0, MonthlyInterval: 0, YearlyInterval: 0,
	})
	d := &model.Device{AssetCode: "MA-ROLL", Name: "DR", Category: "影像设备", Status: constants.DeviceStatusInUse}
	deviceRepo.Create(d)

	// 生成首期。
	if _, err := svc.GeneratePlans("admin"); err != nil {
		t.Fatal(err)
	}
	pending, _, _ := mRepo.List(1, 10, d.ID, constants.MaintenanceTypeDaily, "")
	if len(pending) != 1 {
		t.Fatalf("want 1 daily plan, got %d", len(pending))
	}
	// 开始并在 10 天前完成。
	if _, err := svc.Start(pending[0].ID, &dto.StartMaintenanceReq{Engineer: "工"}, "admin"); err != nil {
		t.Fatal(err)
	}
	completedAt := time.Now().AddDate(0, 0, -10)
	if err := mRepo.DB().Model(&model.MaintenanceRecord{}).Where("id = ?", pending[0].ID).
		Updates(map[string]any{"status": constants.MaintenanceStatusCompleted, "executed_date": completedAt}).Error; err != nil {
		t.Fatal(err)
	}

	// 完成日是 10 天前、周期 1 天 => 计划日期应逾期保留在 9 天前，不前推。
	res, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 1 {
		t.Fatalf("created=%d, want 1", res.Created)
	}
	var next model.MaintenanceRecord
	mRepo.DB().Where("device_id = ? AND type = ? AND status = ?", d.ID, constants.MaintenanceTypeDaily, constants.MaintenanceStatusPending).First(&next)
	want := truncateToDate(completedAt.AddDate(0, 0, 1))
	if next.PlannedDate == nil || !sameDate(*next.PlannedDate, want) {
		t.Fatalf("planned=%v, want overdue date %s", next.PlannedDate, want)
	}
	if !want.Before(truncateToDate(time.Now())) {
		t.Fatalf("expected overdue planned date, got %s", want)
	}
}

func sameDate(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func TestComplete_AutoCreatesNextPeriod(t *testing.T) {
	svc, deviceRepo, mRepo, pRepo := newMaintenanceTestEnv(t)
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "影像设备", DailyInterval: 0, WeeklyInterval: 7, MonthlyInterval: 0, YearlyInterval: 0,
	})
	d := &model.Device{AssetCode: "MA-NEXT", Name: "MRI", Category: "影像设备", Status: constants.DeviceStatusInUse}
	deviceRepo.Create(d)
	if _, err := svc.GeneratePlans("admin"); err != nil {
		t.Fatal(err)
	}
	pending, _, _ := mRepo.List(1, 10, d.ID, constants.MaintenanceTypeWeekly, "")
	if _, err := svc.Start(pending[0].ID, &dto.StartMaintenanceReq{Engineer: "工"}, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Complete(pending[0].ID, &dto.CompleteMaintenanceReq{Content: "已完成周检"}, "admin"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// 完成后应自动顺延出下一期周检计划。
	var n int64
	mRepo.DB().Model(&model.MaintenanceRecord{}).
		Where("device_id = ? AND type = ? AND status = ?", d.ID, constants.MaintenanceTypeWeekly, constants.MaintenanceStatusPending).
		Count(&n)
	if n != 1 {
		t.Fatalf("pending weekly after complete = %d, want 1", n)
	}
	// 重复点击生成不应再多出计划。
	res, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 {
		t.Fatalf("created after rollover = %d, want 0", res.Created)
	}
}

func TestGeneratePlans_ScrappedSkipped(t *testing.T) {
	svc, deviceRepo, _, pRepo := newMaintenanceTestEnv(t)
	pRepo.DB().Create(&model.MaintenancePolicy{
		Category: "影像设备", DailyInterval: 1, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365,
	})
	d := &model.Device{AssetCode: "MA-SCRAP", Name: "旧机", Category: "影像设备", Status: constants.DeviceStatusScrapped}
	deviceRepo.Create(d)
	res, err := svc.GeneratePlans("admin")
	if err != nil {
		t.Fatal(err)
	}
	if res.Created != 0 {
		t.Fatalf("created for scrapped = %d, want 0", res.Created)
	}
}
