// 积分使用数据
export interface IUsageData {
  id: number;
  creditsUsed: number;
  createdAt: string;
  model: string;
}

// 用户配置（API响应）
export interface IUserConfig {
  cookie: boolean;                  // Cookie配置状态
  interval: number;                 // 秒
  timeRange: number;                // 分钟
  enabled: boolean;
  dailyResetUsed: boolean;          // 当日重置是否已使用
}

// 用户配置（API请求）
export interface IUserConfigRequest {
  cookie?: string | undefined;      // Cookie内容（设置时使用，可选字段）
  interval: number;                 // 秒
  timeRange: number;                // 分钟
  enabled: boolean;
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