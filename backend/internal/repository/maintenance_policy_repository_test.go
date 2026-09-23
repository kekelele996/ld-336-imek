package repository

import (
	"testing"

	"github.com/medasset/medasset/internal/model"
	"gorm.io/gorm"
)

func TestMaintenancePolicyUpsert(t *testing.T) {
	db := newTestDB(t)
	repo := NewMaintenancePolicyRepository(db)

	first := model.MaintenancePolicy{Category: "影像设备", DailyInterval: 1, WeeklyInterval: 7, MonthlyInterval: 30, YearlyInterval: 365}
	if err := db.Transaction(func(tx *gorm.DB) error { return repo.UpsertTx(tx, &first) }); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	// 停用日检后再次 upsert，间隔 0 必须落库（不能被默认值覆盖）。
	updated := model.MaintenancePolicy{Category: "影像设备", DailyInterval: 0, WeeklyInterval: 14, MonthlyInterval: 90, YearlyInterval: 365, UpdatedBy: "admin"}
	if err := db.Transaction(func(tx *gorm.DB) error { return repo.UpsertTx(tx, &updated) }); err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	got, err := repo.FindByCategory("影像设备")
	if err != nil {
		t.Fatal(err)
	}
	if got.DailyInterval != 0 {
		t.Errorf("DailyInterval = %d, want 0 (disabled)", got.DailyInterval)
	}
	if got.WeeklyInterval != 14 {
		t.Errorf("WeeklyInterval = %d, want 14", got.WeeklyInterval)
	}
	m, err := repo.MapByCategory()
	if err != nil || len(m) != 1 || m["影像设备"].MonthlyInterval != 90 {
		t.Fatalf("MapByCategory = %v, err=%v", m, err)
	}
}
