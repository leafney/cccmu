import type { IUserConfig, IAPIResponse, IUsageData, ICreditBalance } from '../types';

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
  async updateConfig(config: Partial<IUserConfig>): Promise<IAPIResponse> {
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

  // 获取积分余额
  async getCreditBalance(): Promise<IAPIResponse<ICreditBalance>> {
    return this.request<ICreditBalance>('/balance');
  }


  // 创建SSE连接
  createSSEConnection(
    onMessage: (data: IUsageData[]) => void, 
    onBalanceUpdate?: (balance: ICreditBalance) => void,
    onError?: (error: Event) => void, 
    onOpen?: () => void,
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