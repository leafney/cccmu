import type { IUserConfig, IUserConfigRequest, IAPIResponse, IUsageData, ICreditBalance, IMonitoringStatus } from '../types';

// 认证相关接口类型（内部使用）

interface ILoginResponse {
  message: string;
  expiresAt: string;
}

interface IAuthStatusResponse {
  authenticated: boolean;
  expiresAt?: string;
}

const API_BASE = '/api';
const DEFAULT_TIMEOUT = 30000; // 30秒超时

// 创建超时控制器
function createTimeoutController(timeout: number = DEFAULT_TIMEOUT): AbortController {
  const controller = new AbortController();
  setTimeout(() => controller.abort(), timeout);
  return controller;
}

class APIClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {},
    timeout: number = DEFAULT_TIMEOUT
  ): Promise<IAPIResponse<T>> {
    const controller = createTimeoutController(timeout);
    
    try {
      const response = await fetch(`${API_BASE}${endpoint}`, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...options.headers,
        },
      });

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }

      return response.json();
    } catch (error) {
      if (error instanceof Error && error.name === 'AbortError') {
        throw new Error('请求超时，请检查网络连接');
      }
      throw error;
    }
  }

  // 获取配置
  async getConfig(): Promise<IAPIResponse<IUserConfig>> {
    return this.request<IUserConfig>('/config');
  }

  // 更新配置
  async updateConfig(config: IUserConfigRequest): Promise<IAPIResponse> {
    return this.request('/config', {
      method: 'PUT',
      body: JSON.stringify(config),
    });
  }

  // 启动任务
  async startTask(): Promise<IAPIResponse> {
    return this.request('/control/start', {
      method: 'POST',
    });
  }

  // 停止任务
  async stopTask(): Promise<IAPIResponse> {
    return this.request('/control/stop', {
      method: 'POST',
    });
  }

  // 手动刷新
  async refreshData(): Promise<IAPIResponse> {
    return this.request('/refresh', {
      method: 'POST',
    });
  }

  // 清除Cookie
  async clearCookie(): Promise<IAPIResponse> {
    return this.request('/config/cookie', {
      method: 'DELETE',
    });
  }

  // 登录
  async login(key: string): Promise<IAPIResponse<ILoginResponse>> {
    return this.request<ILoginResponse>('/auth/login', {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${key}`,
      },
    });
  }

  // 登出
  async logout(): Promise<IAPIResponse> {
    return this.request('/auth/logout');
  }

  // 检查认证状态
  async checkAuthStatus(): Promise<IAPIResponse<IAuthStatusResponse>> {
    return this.request<IAuthStatusResponse>('/auth/status');
  }

  // 获取积分余额
  async getCreditBalance(): Promise<IAPIResponse<ICreditBalance>> {
    return this.request<ICreditBalance>('/balance');
  }

  // 重置积分
  async resetCredits(): Promise<IAPIResponse> {
    return this.request('/balance/reset', {
      method: 'POST',
    });
  }


  // 创建SSE连接
  createSSEConnection(
    onMessage: (data: IUsageData[]) => void, 
    onBalanceUpdate?: (balance: ICreditBalance) => void,
    onError?: (error: Event) => void, 
    onOpen?: () => void,
    onResetStatusUpdate?: (resetUsed: boolean) => void,
    onMonitoringStatusUpdate?: (status: IMonitoringStatus) => void,
    onAuthExpired?: () => void,
    timeRange: number = 60
  ): EventSource {
    const eventSource = new EventSource(`${API_BASE}/usage/stream?minutes=${timeRange}`);
    
    eventSource.addEventListener('connected', () => {
      // 连接确认事件
    });

    eventSource.addEventListener('usage', (event) => {
      try {
        const data = JSON.parse(event.data);
        onMessage(data);
      } catch (error) {
        console.error('解析SSE数据失败:', error, event.data);
      }
    });

    eventSource.addEventListener('balance', (event) => {
      try {
        const balance = JSON.parse(event.data);
        if (onBalanceUpdate) {
          onBalanceUpdate(balance);
        }
      } catch (error) {
        console.error('解析积分余额数据失败:', error, event.data);
      }
    });

    eventSource.addEventListener('heartbeat', (event) => {
      // 心跳事件，保持连接活跃
      console.debug('收到SSE心跳:', event.data);
    });

    eventSource.addEventListener('error', (event: MessageEvent) => {
      try {
        const errorData = JSON.parse(event.data);
        console.error('SSE接收到错误信息:', errorData.message);
        // 这里需要在Dashboard组件中显示toast，所以需要传递错误回调
        if (onError && typeof onError === 'function') {
          onError(new CustomEvent('api-error', { detail: errorData.message }) as Event);
        }
      } catch (error) {
        console.error('解析错误信息失败:', error, event.data);
      }
    });

    eventSource.addEventListener('reset_status', (event) => {
      try {
        const resetData = JSON.parse(event.data);
        console.debug('收到重置状态更新:', resetData);
        if (onResetStatusUpdate) {
          onResetStatusUpdate(resetData.resetUsed);
        }
      } catch (error) {
        console.error('解析重置状态数据失败:', error, event.data);
      }
    });

    eventSource.addEventListener('monitoring_status', (event) => {
      try {
        const statusData = JSON.parse(event.data);
        console.debug('收到监控状态更新:', statusData);
        if (onMonitoringStatusUpdate) {
          onMonitoringStatusUpdate(statusData);
        }
      } catch (error) {
        console.error('解析监控状态数据失败:', error, event.data);
      }
    });

    eventSource.addEventListener('auth_expired', (event) => {
      try {
        const authData = JSON.parse(event.data);
        console.warn('认证已过期:', authData.message);
        if (onAuthExpired) {
          onAuthExpired();
        }
      } catch (error) {
        console.error('解析认证过期数据失败:', error, event.data);
      }
    });

    eventSource.onerror = (error) => {
      console.error('SSE连接错误:', error);
      if (onError) {
        onError(error);
      }
    };

    eventSource.onopen = () => {
      if (onOpen) {
        onOpen();
      }
    };
    
    // 添加连接状态监听
    eventSource.onmessage = () => {
      // 默认消息处理
    };

    return eventSource;
  }
}

export const apiClient = new APIClient();