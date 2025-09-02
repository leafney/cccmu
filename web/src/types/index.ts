// 积分使用数据
export interface IUsageData {
  id: number;
  type: string;
  endpoint: string;
  statusCode: number;
  creditsUsed: number;
  createdAt: string;
  model: string;
}

// 用户配置
export interface IUserConfig {
  cookie: string;
  interval: number;                  // 分钟
  timeRange: number;                 // 小时
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