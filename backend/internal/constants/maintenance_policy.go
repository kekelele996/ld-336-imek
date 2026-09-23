package constants

// 默认设备类别（与前端 DEVICE_CATEGORIES 对应，生成默认保养策略时使用）。
var DefaultDeviceCategories = []string{
	"影像设备", "生命支持", "检验设备", "手术器械", "消毒设备", "康复设备", "其他",
}

// 各类别默认保养周期（天），间隔为 0 表示停用该类保养计划。
const (
	DefaultDailyInterval   = 1
	DefaultWeeklyInterval  = 7
	DefaultMonthlyInterval = 30
	DefaultYearlyInterval  = 365
)

// 保养周期间隔允许的最大值（天）。
const MaxMaintenanceIntervalDays = 3650
