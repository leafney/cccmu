// 积分使用数据
export interface IUsageData {
  id: number;
  creditsUsed: number;
  createdAt: string;
  model: string;
}

// 自动调度配置
export interface IAutoScheduleConfig {
  enabled: boolean;           // 是否启用自动调度
  startTime: string;          // 开启时间 "HH:MM"
  endTime: string;            // 关闭时间 "HH:MM"
  monitoringOn: boolean;      // 时间范围内是开启还是关闭监控
}

// 自动重置配置
export interface IAutoResetConfig {
  enabled: boolean;          // 是否启用自动重置
  timeEnabled: boolean;      // 时间触发条件是否启用
  resetTime: string;         // 重置时间 "HH:MM"
  thresholdEnabled: boolean; // 积分阈值触发是否启用
  threshold: number;         // 积分阈值
  thresholdTimeEnabled: boolean; // 阈值时间范围是否启用
  thresholdStartTime: string;    // 阈值检查开始时间 "HH:MM"
  thresholdEndTime: string;      // 阈值检查结束时间 "HH:MM"
}

// 版本信息
export interface IVersionInfo {
  version: string;   // 版本号
  gitCommit: string; // Git提交短哈希
  buildTime: string; // 构建时间
  goVersion: string; // Go版本
}

// 用户配置（API响应）
export interface IUserConfig {
  cookie: boolean;                  // Cookie配置状态
  interval: number;                 // 秒
  timeRange: number;                // 分钟
  enabled: boolean;
  dailyResetUsed: boolean;          // 当日重置是否已使用
  autoSchedule: IAutoScheduleConfig; // 自动调度配置
  autoReset: IAutoResetConfig;       // 自动重置配置
  version: IVersionInfo;            // 版本信息
}

// 用户配置（API请求）
export interface IUserConfigRequest {
  cookie?: string | undefined;      // Cookie内容（设置时使用，可选字段）
  interval: number;                 // 秒
  timeRange: number;                // 分钟
  enabled: boolean;
  autoSchedule?: IAutoScheduleConfig; // 自动调度配置（可选）
  autoReset?: IAutoResetConfig;       // 自动重置配置（可选）
}

// API响应格式
export interface IAPIResponse<T = any> {
  code: number;
  message: string;
  data?: T;
}

// 错误响应
export interface IErrorResponse {
  code: number;
  message: string;
  error?: string;
}

// SSE事件类型
export interface ISSEEvent {
  type: 'usage' | 'config' | 'status';
  data: any;
  timestamp: string;
}

// 图表数据点
export interface IChartDataPoint {
  timestamp: string;
  creditsUsed: number;
  model: string;
}

// 积分余额信息
export interface ICreditBalance {
  remaining: number;
  updatedAt: string;
}

// 监控状态信息（SSE推送）
export interface IMonitoringStatus {
  type: 'monitoring_status';
  isMonitoring: boolean;          // 当前监控是否运行
  autoScheduleEnabled: boolean;   // 自动调度是否启用
  autoScheduleActive: boolean;    // 当前是否在自动调度时间范围内
  timestamp: string;
}