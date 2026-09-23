// 保养周期策略表格列定义：与后端四类保养（日/周/月/年检）对应。
export interface PolicyTypeColumn {
  key: 'daily_interval' | 'weekly_interval' | 'monthly_interval' | 'yearly_interval';
  label: string;
  defaultInterval: number;
}

export const POLICY_TYPE_COLUMNS: PolicyTypeColumn[] = [
  { key: 'daily_interval', label: '日检', defaultInterval: 1 },
  { key: 'weekly_interval', label: '周检', defaultInterval: 7 },
  { key: 'monthly_interval', label: '月检', defaultInterval: 30 },
  { key: 'yearly_interval', label: '年检', defaultInterval: 365 },
];
