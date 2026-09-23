package constants

// 设备状态枚举。
const (
	DeviceStatusInStorage       = "in_storage"        // 在库
	DeviceStatusInUse           = "in_use"            // 使用中
	DeviceStatusUnderMaintenance = "under_maintenance" // 维修中
	DeviceStatusDisabled        = "disabled"          // 已禁用
	DeviceStatusScrapped        = "scrapped"          // 已报废
)

// 采购申请状态枚举。
const (
	PurchaseStatusPendingDeviceAdmin = "pending_device_admin" // 待设备科审核
	PurchaseStatusPendingDean        = "pending_dean"         // 待院长审批
	PurchaseStatusApproved           = "approved"             // 审批通过
	PurchaseStatusDelivered          = "delivered"            // 已到货待验收
	PurchaseStatusAccepted           = "accepted"             // 已验收
	PurchaseStatusRejected           = "rejected"             // 已拒绝
)

// 保养/维修类型枚举。
const (
	MaintenanceTypeDaily   = "daily"   // 日检
	MaintenanceTypeWeekly  = "weekly"  // 周检
	MaintenanceTypeMonthly = "monthly" // 月检
	MaintenanceTypeYearly  = "yearly"  // 年检
	MaintenanceTypeRepair  = "repair"  // 故障维修
)

// 保养/维修状态枚举。
const (
	MaintenanceStatusPending    = "pending"    // 待处理
	MaintenanceStatusInProgress = "in_progress" // 处理中
	MaintenanceStatusCompleted  = "completed"  // 已完成
	MaintenanceStatusCancelled  = "cancelled"  // 已取消
)

// MaintenancePlanTypes 周期策略可配置的保养类型（日检/周检/月检/年检，不含故障维修）。
// 生成计划、策略启用校验、默认策略初始化均复用此切片。
var MaintenancePlanTypes = []string{
	MaintenanceTypeDaily,
	MaintenanceTypeWeekly,
	MaintenanceTypeMonthly,
	MaintenanceTypeYearly,
}

// DefaultMaintenanceIntervalDays 保养类型默认周期（天）：日检1天、周检7天、月检30天、年检365天。
var DefaultMaintenanceIntervalDays = map[string]int{
	MaintenanceTypeDaily:   1,
	MaintenanceTypeWeekly:  7,
	MaintenanceTypeMonthly: 30,
	MaintenanceTypeYearly:  365,
}

// DefaultDeviceCategories 平台内置设备类别（与前端 DEVICE_CATEGORIES 对应），
// 用于启动时为每个类别初始化四类保养周期策略。
var DefaultDeviceCategories = []string{
	"影像设备", "生命支持", "检验设备", "手术器械", "消毒设备", "康复设备", "其他",
}

// 计量状态枚举。
const (
	CalibrationStatusNormal     = "normal"      // 合格
	CalibrationStatusUnqualified = "unqualified" // 不合格
	CalibrationStatusDue        = "due"         // 即将到期
	CalibrationStatusExpired    = "expired"     // 已过期
)

// 调拨状态枚举。
const (
	TransferStatusPending  = "pending"
	TransferStatusApproved = "approved"
	TransferStatusRejected = "rejected"
)

// 报废状态枚举。
const (
	ScrapStatusPending  = "pending"
	ScrapStatusApproved = "approved"
	ScrapStatusRejected = "rejected"
)

// 用户状态枚举。
const (
	UserStatusActive   = "active"
	UserStatusDisabled = "disabled"
)

// 计量结果枚举。
const (
	CalibrationResultQualified   = "qualified"
	CalibrationResultUnqualified = "unqualified"
)
