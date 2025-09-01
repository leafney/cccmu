import { useState, useEffect, useCallback, useRef } from 'react';
import { UsageChart } from '../components/UsageChart';
import { SettingsModal } from '../components/SettingsModal';
import type { IUsageData, IUserConfig, ICreditBalance } from '../types';
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
  const [creditBalance, setCreditBalance] = useState<ICreditBalance | null>(null);
  const retryTimeoutRef = useRef<number | null>(null);

  // 建立SSE连接
  const connectSSE = useCallback(() => {
    // 清理现有的重试计时器
    if (retryTimeoutRef.current) {
      clearTimeout(retryTimeoutRef.current);
      retryTimeoutRef.current = null;
    }

    // 关闭现有连接
    setEventSource(prev => {
      if (prev) {
        prev.close();
        setIsConnected(false); // 关闭连接时重置状态
      }
      return null;
    });

    const timeRange = config?.timeRange || 1;
    
    const newEventSource = apiClient.createSSEConnection(
      (data: IUsageData[]) => {
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
          connectSSE();
        }, 5000);
      },
      () => {
        // SSE连接成功时设置连接状态
        setIsConnected(true);
      },
      timeRange
    );

    setEventSource(newEventSource);
  }, [config?.timeRange]);

  // 获取积分余额
  const loadCreditBalance = useCallback(async () => {
    try {
      const response = await apiClient.getCreditBalance();
      if (response.data) {
        setCreditBalance(response.data);
      }
    } catch (error) {
      console.error('获取积分余额失败:', error);
    }
  }, []);

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

        // 加载积分余额
        await loadCreditBalance();
      } catch (error) {
        console.error('加载配置和状态失败:', error);
      }
    };

    loadConfigAndStatus();
  }, [loadCreditBalance]);

  // 配置更新后重新连接SSE - 只在timeRange变化时重连
  useEffect(() => {
    if (config) {
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
        eventSource.close();
        setIsConnected(false);
      }
    };
  }, [config?.timeRange, connectSSE]); // 只在timeRange变化时重连

  // 处理配置更新
  const handleConfigUpdate = async (newConfig: IUserConfig) => {
    setConfig(newConfig);
    
    // 检查实际的任务运行状态，确保状态同步
    try {
      const statusResponse = await fetch('/api/control/status');
      const statusResult = await statusResponse.json();
      if (statusResult.data) {
        setIsMonitoring(statusResult.data.running);
      } else {
        setIsMonitoring(newConfig.enabled);
      }
    } catch (error) {
      console.error('检查任务状态失败:', error);
      setIsMonitoring(newConfig.enabled);
    }
  };

  // 获取历史数据
  const loadHistoricalData = useCallback(async () => {
    try {
      const minutes = config?.timeRange || 60;
      const response = await fetch(`/api/usage/data?minutes=${minutes}`);
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

  // 定时刷新积分余额
  useEffect(() => {
    if (config?.cookie && config.enabled) {
      const interval = setInterval(() => {
        loadCreditBalance();
      }, 2 * 60 * 1000); // 每2分钟刷新一次积分余额

      return () => clearInterval(interval);
    }
  }, [config?.cookie, config?.enabled, loadCreditBalance]);

  const toggleMonitoring = async () => {
    try {
      if (isMonitoring) {
        // 先立即更新UI状态为关闭，提供即时反馈
        setIsMonitoring(false);
        
        try {
          await apiClient.stopTask();
          
          // 停止任务后检查实际状态
          const statusResponse = await fetch('/api/control/status');
          const statusResult = await statusResponse.json();
          if (statusResult.data) {
            // 如果实际状态与UI状态不一致，恢复UI状态
            if (statusResult.data.running !== false) {
              setIsMonitoring(statusResult.data.running);
            }
          }
          
          // 同步更新配置中的enabled状态
          if (config) {
            const updatedConfig = { ...config, enabled: false };
            await apiClient.updateConfig(updatedConfig);
            setConfig(updatedConfig);
          }
        } catch (error) {
          // 停止操作失败，恢复UI状态为开启
          console.error('停止监控失败:', error);
          setIsMonitoring(true);
          throw error;
        }
      } else {
        // 启动监控时先立即更新UI状态
        setIsMonitoring(true);
        
        try {
          await apiClient.startTask();
          
          // 启动任务后检查实际状态
          const statusResponse = await fetch('/api/control/status');
          const statusResult = await statusResponse.json();
          if (statusResult.data) {
            // 如果实际状态与UI状态不一致，恢复UI状态
            if (statusResult.data.running !== true) {
              setIsMonitoring(statusResult.data.running);
            }
          }
          
          // 同步更新配置中的enabled状态
          if (config) {
            const updatedConfig = { ...config, enabled: true };
            await apiClient.updateConfig(updatedConfig);
            setConfig(updatedConfig);
            // 配置更新后，useEffect会自动触发loadHistoricalData，无需重复调用
          }
        } catch (error) {
          // 启动操作失败，恢复UI状态为关闭
          console.error('启动监控失败:', error);
          setIsMonitoring(false);
          throw error;
        }
      }
    } catch (error) {
      // 最终错误处理：重新加载实际状态
      try {
        const statusResponse = await fetch('/api/control/status');
        const statusResult = await statusResponse.json();
        if (statusResult.data) {
          setIsMonitoring(statusResult.data.running);
        }
      } catch (statusError) {
        console.error('重新加载状态失败:', statusError);
      }
    }
  };

  // 手动刷新数据
  const handleRefresh = async () => {
    // 检查Cookie是否存在
    if (!config?.cookie || config.cookie === '') {
      console.warn('无法刷新数据：未配置Cookie');
      return;
    }

    try {
      // 同时刷新使用数据和积分余额
      await Promise.all([
        fetch('/api/refresh', { method: 'POST' }),
        apiClient.refreshBalance()
      ]);
      
      await Promise.all([
        loadHistoricalData(),
        loadCreditBalance()
      ]);
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
          <h1 className="text-xl md:text-2xl font-bold">Claude 积分监控</h1>
          <p className="text-sm text-white/70 mt-1">
            {lastUpdate ? `最后更新: ${lastUpdate.toLocaleTimeString()}` : '等待数据...'}
          </p>
        </div>

        {/* 右侧控制区 */}
        <div className="flex items-center space-x-4">
          {/* 积分余额显示 */}
          {creditBalance && (
            <div className="text-white bg-white/10 px-3 py-1 rounded-lg backdrop-blur-sm">
              <div className="text-xs text-white/70">剩余积分约为</div>
              <div className="text-sm font-mono font-bold text-yellow-400">
                {creditBalance.remaining.toLocaleString()}
              </div>
            </div>
          )}
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
            disabled={!config?.cookie || config.cookie === ''}
            className="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            title={!config?.cookie || config.cookie === '' ? "请先配置Cookie" : "手动刷新数据"}
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