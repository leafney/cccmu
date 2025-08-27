import { useState, useEffect, useCallback, useRef } from 'react';
import { UsageChart } from '../components/UsageChart';
import { SettingsModal } from '../components/SettingsModal';
import type { IUsageData, IUserConfig } from '../types';
import { apiClient } from '../api/client';
import { Settings, Wifi, WifiOff, RefreshCw } from 'lucide-react';

export function Dashboard() {
  const [usageData, setUsageData] = useState<IUsageData[]>([]);
  const [config, setConfig] = useState<IUserConfig | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [isMonitoring, setIsMonitoring] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [eventSource, setEventSource] = useState<EventSource | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const retryTimeoutRef = useRef<number | null>(null);

  // 建立SSE连接
  const connectSSE = useCallback(() => {
    console.log('开始建立SSE连接...');
    
    // 清理现有的重试计时器
    if (retryTimeoutRef.current) {
      clearTimeout(retryTimeoutRef.current);
      retryTimeoutRef.current = null;
    }

    // 关闭现有连接
    setEventSource(prev => {
      if (prev) {
        console.log('关闭现有SSE连接');
        prev.close();
        setIsConnected(false); // 关闭连接时重置状态
      }
      return null;
    });

    const timeRange = config?.timeRange || 1;
    console.log(`创建SSE连接，时间范围: ${timeRange}小时`);
    
    const newEventSource = apiClient.createSSEConnection(
      (data: IUsageData[]) => {
        console.log('SSE接收到数据，设置连接状态为已连接');
        setUsageData(data);
        setLastUpdate(new Date());
        // 收到数据时确保连接状态为已连接
        setIsConnected(true);
      },
      (error: Event) => {
        console.error('SSE连接错误:', error);
        setIsConnected(false);
        // 5秒后重试连接
        retryTimeoutRef.current = setTimeout(() => {
          console.log('SSE连接错误，5秒后重试');
          connectSSE();
        }, 5000);
      },
      () => {
        // SSE连接成功时设置连接状态
        console.log('SSE连接建立成功，设置连接状态为已连接');
        setIsConnected(true);
      },
      timeRange
    );

    setEventSource(newEventSource);
  }, [config?.timeRange]);

  // 加载初始配置和任务状态
  useEffect(() => {
    const loadConfigAndStatus = async () => {
      try {
        // 加载配置
        const configResponse = await apiClient.getConfig();
        if (configResponse.data) {
          setConfig(configResponse.data);
        }

        // 加载任务运行状态
        const statusResponse = await fetch('/api/control/status');
        const statusResult = await statusResponse.json();
        if (statusResult.data) {
          setIsMonitoring(statusResult.data.running);
        }
      } catch (error) {
        console.error('加载配置和状态失败:', error);
      }
    };

    loadConfigAndStatus();
  }, []);

  // 配置更新后重新连接SSE - 只在timeRange变化时重连
  useEffect(() => {
    if (config) {
      console.log('配置已加载，建立SSE连接');
      connectSSE();
    }

    return () => {
      // 清理重试计时器
      if (retryTimeoutRef.current) {
        clearTimeout(retryTimeoutRef.current);
        retryTimeoutRef.current = null;
      }
      // 关闭SSE连接
      if (eventSource) {
        console.log('组件卸载，关闭SSE连接');
        eventSource.close();
        setIsConnected(false);
      }
    };
  }, [config?.timeRange, connectSSE]); // 只在timeRange变化时重连

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

  const toggleMonitoring = async () => {
    try {
      if (isMonitoring) {
        await apiClient.stopTask();
        setIsMonitoring(false);
      } else {
        await apiClient.startTask();
        setIsMonitoring(true);
        
        // 监控启动后立即获取一次数据
        setTimeout(async () => {
          try {
            await loadHistoricalData();
          } catch (error) {
            console.error('立即获取数据失败:', error);
          }
        }, 500); // 延迟500ms确保后端任务已启动
      }
    } catch (error) {
      console.error('切换监控状态失败:', error);
    }
  };

  // 手动刷新数据
  const handleRefresh = async () => {
    try {
      await fetch('/api/refresh', { method: 'POST' });
      await loadHistoricalData();
    } catch (error) {
      console.error('手动刷新失败:', error);
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-blue-900 to-purple-900">
      {/* 顶部控制栏 */}
      <div className="absolute top-0 left-0 right-0 z-10 flex items-center justify-between p-4 md:p-6">
        {/* 左侧标题 */}
        <div className="text-white">
          <h1 className="text-xl md:text-2xl font-bold">Claude Code 积分监控</h1>
          <p className="text-sm text-white/70 mt-1">
            {lastUpdate ? `最后更新: ${lastUpdate.toLocaleTimeString()}` : '等待数据...'}
          </p>
        </div>

        {/* 右侧控制区 */}
        <div className="flex items-center space-x-4">
          {/* 连接状态指示 */}
          <div className="flex items-center space-x-2">
            {isConnected ? (
              <Wifi className="w-5 h-5 text-green-400" />
            ) : (
              <WifiOff className="w-5 h-5 text-red-400" />
            )}
            <span className="text-sm text-white/80 hidden md:block">
              {isConnected ? '已连接' : '连接断开'}
            </span>
          </div>

          {/* 监控状态开关 */}
          <div className="flex items-center space-x-2">
            <span className="text-sm text-white/80 hidden md:block">监控</span>
            <button
              onClick={toggleMonitoring}
              disabled={!config?.cookie || config.cookie === ''}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-white/20 disabled:opacity-50 ${
                isMonitoring ? 'bg-green-500' : 'bg-gray-600'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition ${
                  isMonitoring ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
          </div>

          {/* 手动刷新按钮 */}
          <button
            onClick={handleRefresh}
            className="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
            title="手动刷新数据"
          >
            <RefreshCw className="w-5 h-5" />
          </button>

          {/* 设置按钮 */}
          <button
            onClick={() => setShowSettings(true)}
            className="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors"
            title="打开设置"
          >
            <Settings className="w-5 h-5" />
          </button>
        </div>
      </div>

      {/* 全屏图表区域 */}
      <div className="h-screen w-full flex items-center justify-center p-4 pt-20">
        <div className="w-full h-full max-h-[calc(100vh-100px)]">
          <UsageChart data={usageData} className="h-full" />
        </div>
      </div>

      {/* 设置模态弹窗 */}
      <SettingsModal
        isOpen={showSettings}
        onClose={() => setShowSettings(false)}
        onConfigUpdate={handleConfigUpdate}
      />
    </div>
  );
}