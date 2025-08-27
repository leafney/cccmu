import type { IUserConfig, IAPIResponse, IUsageData } from '../types';

const API_BASE = '/api';

class APIClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<IAPIResponse<T>> {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP ${response.status}: ${response.statusText}`);
    }

    return response.json();
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

  // 创建SSE连接
  createSSEConnection(onMessage: (data: IUsageData[]) => void, onError?: (error: Event) => void, timeRange: number = 1): EventSource {
    const eventSource = new EventSource(`${API_BASE}/usage/stream?hours=${timeRange}`);
    
    eventSource.addEventListener('usage', (event) => {
      try {
        console.log('收到SSE usage事件:', event.data);
        const data = JSON.parse(event.data);
        console.log('解析后的数据:', data);
        onMessage(data);
      } catch (error) {
        console.error('解析SSE数据失败:', error, event.data);
      }
    });

    eventSource.addEventListener('heartbeat', (event) => {
      // 心跳事件，保持连接活跃
      console.debug('收到SSE心跳:', event.data);
    });

    eventSource.onerror = (error) => {
      console.error('SSE连接错误:', error);
      if (onError) {
        onError(error);
      }
    };

    eventSource.onopen = () => {
      console.log('SSE连接已建立');
    };

    return eventSource;
  }
}

export const apiClient = new APIClient();