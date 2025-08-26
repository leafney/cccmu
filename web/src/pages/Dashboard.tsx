import { useState, useEffect, useCallback } from 'react';
import { UsageChart } from '../components/UsageChart';
import { SettingsPanel } from '../components/SettingsPanel';
import { StatusIndicator } from '../components/StatusIndicator';
import type { IUsageData, IUserConfig } from '../types';
import { apiClient } from '../api/client';

export function Dashboard() {
  const [usageData, setUsageData] = useState<IUsageData[]>([]);
  const [config, setConfig] = useState<IUserConfig | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [isMonitoring, setIsMonitoring] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [eventSource, setEventSource] = useState<EventSource | null>(null);

  // 建立SSE连接
  const connectSSE = useCallback(() => {
    if (eventSource) {
      eventSource.close();
    }

    const newEventSource = apiClient.createSSEConnection(
      (data: IUsageData[]) => {
        setUsageData(data);
        setLastUpdate(new Date());
        setIsConnected(true);
      },
      (error: Event) => {
        console.error('SSE连接错误:', error);
        setIsConnected(false);
        // 5秒后重试连接
        setTimeout(connectSSE, 5000);
      }
    );

    newEventSource.onopen = () => {
      console.log('SSE连接已建立');
      setIsConnected(true);
    };

    // EventSource 没有 onclose 事件，这里不需要设置

    setEventSource(newEventSource);
  }, [config?.timeRange, eventSource]);

  // 加载初始配置
  useEffect(() => {
    const loadConfig = async () => {
      try {
        const response = await apiClient.getConfig();
        if (response.data) {
          setConfig(response.data);
          setIsMonitoring(response.data.enabled);
        }
      } catch (error) {
        console.error('加载配置失败:', error);
      }
    };

    loadConfig();
  }, []);

  // 配置更新后重新连接SSE
  useEffect(() => {
    if (config) {
      connectSSE();
    }

    return () => {
      if (eventSource) {
        eventSource.close();
      }
    };
  }, [config]);

  // 处理配置更新
  const handleConfigUpdate = (newConfig: IUserConfig) => {
    setConfig(newConfig);
    setIsMonitoring(newConfig.enabled);
  };

  // 获取历史数据
  const loadHistoricalData = useCallback(async () => {
    try {
      const hours = config?.timeRange || 1;
      const response = await fetch(`/api/usage/data?hours=${hours}`);
      const result = await response.json();
      if (result.data) {
        setUsageData(result.data);
        setLastUpdate(new Date());
      }
    } catch (error) {
      console.error('加载历史数据失败:', error);
    }
  }, [config?.timeRange]);

  // 初始加载历史数据
  useEffect(() => {
    if (config) {
      loadHistoricalData();
    }
  }, [config, loadHistoricalData]);

  return (
    <div className="min-h-screen bg-gray-50 p-4">
      <div className="max-w-7xl mx-auto">
        {/* 页面标题 */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-gray-900">Claude Code积分监控</h1>
          <p className="mt-2 text-gray-600">实时监控Claude Code的积分使用情况</p>
        </div>

        {/* 主要内容区域 */}
        <div className="grid grid-cols-1 lg:grid-cols-4 gap-6">
          {/* 左侧主要区域 - 图表 */}
          <div className="lg:col-span-3">
            <UsageChart data={usageData} className="mb-6" />
            
            {/* 数据统计卡片 */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-white rounded-lg shadow-sm border p-6">
                <div className="text-2xl font-bold text-blue-600">
                  {usageData.reduce((sum, item) => sum + item.creditsUsed, 0)}
                </div>
                <div className="text-sm text-gray-600">总积分使用</div>
              </div>
              
              <div className="bg-white rounded-lg shadow-sm border p-6">
                <div className="text-2xl font-bold text-green-600">
                  {usageData.length}
                </div>
                <div className="text-sm text-gray-600">API调用次数</div>
              </div>
              
              <div className="bg-white rounded-lg shadow-sm border p-6">
                <div className="text-2xl font-bold text-purple-600">
                  {new Set(usageData.map(item => item.model)).size}
                </div>
                <div className="text-sm text-gray-600">使用模型数</div>
              </div>
            </div>
          </div>

          {/* 右侧侧边栏 */}
          <div className="lg:col-span-1 space-y-6">
            {/* 状态指示器 */}
            <StatusIndicator
              isConnected={isConnected}
              isMonitoring={isMonitoring}
              lastUpdate={lastUpdate || undefined}
            />
            
            {/* 设置面板 */}
            <SettingsPanel onConfigUpdate={handleConfigUpdate} />
          </div>
        </div>

        {/* 底部数据表格 */}
        {usageData.length > 0 && (
          <div className="mt-8 bg-white rounded-lg shadow-sm border overflow-hidden">
            <div className="px-6 py-4 border-b border-gray-200">
              <h3 className="text-lg font-medium text-gray-900">最近使用记录</h3>
            </div>
            <div className="overflow-x-auto">
              <table className="min-w-full divide-y divide-gray-200">
                <thead className="bg-gray-50">
                  <tr>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      时间
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      模型
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      积分
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      状态
                    </th>
                    <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                      端点
                    </th>
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {usageData.slice(0, 10).map((item, index) => (
                    <tr key={`${item.id}-${index}`} className={index % 2 === 0 ? 'bg-white' : 'bg-gray-50'}>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {new Date(item.createdAt).toLocaleString()}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className="inline-flex px-2 py-1 text-xs font-medium bg-blue-100 text-blue-800 rounded-full">
                          {item.model}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-900">
                        {item.creditsUsed}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap">
                        <span className={`inline-flex px-2 py-1 text-xs font-medium rounded-full ${
                          item.statusCode === 200 
                            ? 'bg-green-100 text-green-800' 
                            : 'bg-red-100 text-red-800'
                        }`}>
                          {item.statusCode}
                        </span>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500 truncate max-w-xs">
                        {item.endpoint}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}