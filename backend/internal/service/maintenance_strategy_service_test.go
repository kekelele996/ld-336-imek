package service

import (
	"testing"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/pkg/pointerx"
)

func newStrategySvc(env *testEnv) *MaintenanceStrategyService {
	return NewMaintenanceStrategyService(repository.NewMaintenanceStrategyRepository(env.db), env.audit, env.logger)
}

// TestStrategyCreateDuplicate 同类别同类型策略唯一。
func TestStrategyCreateDuplicate(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newStrategySvc(env)

	if _, err := svc.Create(&dto.CreateMaintenanceStrategyReq{Category: "生命支持", Type: constants.MaintenanceTypeDaily, IntervalDays: 1}, "admin"); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, err := svc.Create(&dto.CreateMaintenanceStrategyReq{Category: "生命支持", Type: constants.MaintenanceTypeDaily, IntervalDays: 2}, "admin"); err == nil {
		t.Error("expected duplicate strategy error")
	}
	// 故障维修不可配置周期策略。
	if _, err := svc.Create(&dto.CreateMaintenanceStrategyReq{Category: "生命支持", Type: constants.MaintenanceTypeRepair, IntervalDays: 1}, "admin"); err == nil {
		t.Error("expected invalid type error for repair")
	}
}

// TestStrategyUpdateToggle 调整周期与停用/启用。
func TestStrategyUpdateToggle(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newStrategySvc(env)

	m, err := svc.Create(&dto.CreateMaintenanceStrategyReq{Category: "影像设备", Type: constants.MaintenanceTypeWeekly, IntervalDays: 7}, "admin")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	updated, err := svc.Update(m.ID, &dto.UpdateMaintenanceStrategyReq{IntervalDays: pointerx.IntPtr(14), Enabled: pointerx.BoolPtr(false), Remark: "调整为双周"}, "admin")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if updated.IntervalDays != 14 || updated.Enabled {
		t.Errorf("interval=%d enabled=%t, want 14/false", updated.IntervalDays, updated.Enabled)
	}
	enabled, err := svc.Update(m.ID, &dto.UpdateMaintenanceStrategyReq{Enabled: pointerx.BoolPtr(true)}, "admin")
	if err != nil {
		t.Fatalf("re-enable failed: %v", err)
	}
	if !enabled.Enabled {
		t.Error("expected enabled=true")
	}
	if _, err := svc.Update(999, &dto.UpdateMaintenanceStrategyReq{}, "admin"); err == nil {
		t.Error("expected not found error")
	}
}

// TestStrategySeedDefaultsIdempotent 默认策略初始化幂等且不覆盖人工调整。
func TestStrategySeedDefaultsIdempotent(t *testing.T) {
	env := newTestServiceEnv(t)
	svc := newStrategySvc(env)

	if err := svc.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults failed: %v", err)
	}
	items, err := svc.List("", nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	want := len(constants.DefaultDeviceCategories) * len(constants.MaintenancePlanTypes)
	if len(items) != want {
		t.Fatalf("seeded = %d, want %d", len(items), want)
	}

	// 人工停用一条后再次初始化：总数不变，停用状态不被覆盖。
	var targetID uint
	for _, it := range items {
		if it.Category == constants.DefaultDeviceCategories[0] && it.Type == constants.MaintenanceTypeDaily {
			targetID = it.ID
		}
	}
	if _, err := svc.Update(targetID, &dto.UpdateMaintenanceStrategyReq{Enabled: pointerx.BoolPtr(false)}, "admin"); err != nil {
		t.Fatalf("disable failed: %v", err)
	}
	if err := svc.SeedDefaults(); err != nil {
		t.Fatalf("SeedDefaults second run failed: %v", err)
	}
	items, err = svc.List("", nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(items) != want {
		t.Errorf("after reseed = %d, want %d（幂等）", len(items), want)
	}
	for _, it := range items {
		if it.ID == targetID && it.Enabled {
			t.Error("disabled strategy was overwritten by reseed")
		}
	}
}
