package service

import (
	"testing"

	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/repository"
)

func newPolicyTestEnv(t *testing.T) *MaintenancePolicyService {
	env := newTestServiceEnv(t)
	return NewMaintenancePolicyService(
		repository.NewMaintenancePolicyRepository(env.db),
		repository.NewDeviceRepository(env.db),
		env.audit, env.logger)
}

func TestPolicyListShowsDefaults(t *testing.T) {
	svc := newPolicyTestEnv(t)
	items, err := svc.List("admin")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) == 0 {
		t.Fatal("expected default categories in list")
	}
	var found bool
	for _, it := range items {
		if it.Category == "影像设备" {
			found = true
			if !it.DailyEnabled || it.DailyInterval != 1 {
				t.Errorf("default daily = %d enabled=%v", it.DailyInterval, it.DailyEnabled)
			}
		}
	}
	if !found {
		t.Fatal("影像设备 default policy missing")
	}
}

func TestPolicyUpdateDisableAndValidate(t *testing.T) {
	svc := newPolicyTestEnv(t)
	// 非法周期被拒。
	bad := &dto.UpdatePolicyReq{Items: []dto.PolicyItem{{Category: "影像设备", DailyInterval: 99999, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365}}}
	if _, err := svc.Update(bad, "admin"); err == nil {
		t.Fatal("expected validation error for interval > 3650")
	}
	// 停用全部周期可保存。
	ok := &dto.UpdatePolicyReq{Items: []dto.PolicyItem{{Category: "影像设备", DailyInterval: 0, WeeklyInterval: 0, MonthlyInterval: 0, YearlyInterval: 0}}}
	items, err := svc.Update(ok, "admin")
	if err != nil {
		t.Fatalf("disable all: %v", err)
	}
	for _, it := range items {
		if it.Category == "影像设备" && (it.DailyEnabled || it.WeeklyEnabled || it.MonthlyEnabled || it.YearlyEnabled) {
			t.Fatalf("all types should be disabled: %+v", it)
		}
	}
}
