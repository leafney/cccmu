import { useState, useEffect, useCallback, useRef } from 'react';
import { UsageChart } from '../components/UsageChart';
import { SettingsModal } from '../components/SettingsModal';
import type { IUsageData, IUserConfig, ICreditBalance } from '../types';
import { apiClient } from '../api/client';
import { Settings, Wifi, WifiOff, RefreshCw } from 'lucide-react';
import toast from 'react-hot-toast';

export function Dashboard() {
  const [usageData, setUsageData] = useState<IUsageData[]>([]);
  const [config, setConfig] = useState<IUserConfig | null>(null);
  const [isConnected, setIsConnected] = useState(false);
  const [isMonitoring, setIsMonitoring] = useState(false);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const [eventSource, setEventSource] = useState<EventSource | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [creditBalance, setCreditBalance] = useState<ICreditBalance | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
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
      (balance: ICreditBalance) => {
        setCreditBalance(balance);
      },
      (error: Event) => {
        console.error('SSE连接错误:', error);
        
        // 检查是否是API错误事件
        if (error.type === 'api-error') {
          const customEvent = error as CustomEvent;
          toast.error(customEvent.detail);
          return; // API错误不需要重新连接
        }
        
        setIsConnected(false);
        
        // SSE断开时检查后端任务状态，如果任务停止则重置监控开关
        const checkTaskStatus = async () => {
          try {
            const statusResponse = await fetch('/api/control/status');
            const statusResult = await statusResponse.json();
            if (statusResult.data && !statusResult.data.running) {
              // 后端任务已停止，重置UI开关状态
              setIsMonitoring(false);
            }
          } catch (error) {
            console.error('检查任务状态失败:', error);
          }
        };
        
        checkTaskStatus();
        
        // 5秒后重试连接
        retryTimeoutRef.current = setTimeout(() => {
          connectSSE();
        }, 5000);
      },
      () => {
        // SSE连接成功时设置连接状态并同步任务状态
        setIsConnected(true);
        
        // 连接成功后检查后端任务状态，确保UI状态同步
        const syncTaskStatus = async () => {
          try {
            const statusResponse = await fetch('/api/control/status');
            const statusResult = await statusResponse.json();
            if (statusResult.data) {
              setIsMonitoring(statusResult.data.running);
            }
          } catch (error) {
            console.error('同步任务状态失败:', error);
          }
        };
        
        syncTaskStatus();
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

        // 初始化时不立即获取数据，等待SSE连接建立后再获取
        // 数据获取将在SSE连接建立后由useEffect触发
      } catch (error) {
        console.error('加载配置和状态失败:', error);
      }
    };

    loadConfigAndStatus();
  }, []);

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

  // 移除未使用的triggerDataLoad函数，现在只通过后端Start方法自动触发

  // SSE连接状态监听（不自动获取数据，等待用户操作）
  // 移除自动数据加载，只有用户主动操作时才获取数据


  const toggleMonitoring = async () => {
    try {
      if (isMonitoring) {
        // 先立即更新UI状态为关闭，提供即时反馈
        setIsMonitoring(false);
        
        try {
          try {
            await apiClient.stopTask();
          } catch (error) {
            console.error('停止监控失败:', error);
            toast.error(error instanceof Error ? error.message : '停止监控失败');
            throw error;
          }
          
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
            try {
              await apiClient.updateConfig(updatedConfig);
            } catch (error) {
              console.error('更新配置失败:', error);
              toast.error(error instanceof Error ? error.message : '更新配置失败');
              throw error;
            }
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
          try {
            await apiClient.startTask();
          } catch (error) {
            console.error('启动监控失败:', error);
            toast.error(error instanceof Error ? error.message : '启动监控失败');
            throw error;
          }
          
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
            try {
              await apiClient.updateConfig(updatedConfig);
            } catch (error) {
              console.error('更新配置失败:', error);
              toast.error(error instanceof Error ? error.message : '更新配置失败');
              throw error;
            }
            setConfig(updatedConfig);
            // 启动监控后不需要手动触发，后端Start方法会自动立即执行一次
          }
        } catch (startError) {
          // 启动操作失败，恢复UI状态为关闭
          console.error('启动监控失败:', startError);
          setIsMonitoring(false);
          throw startError;
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
    // 检查SSE连接状态
    if (!isConnected) {
      toast.error('请等待连接建立');
      return;
    }

    // 检查Cookie是否存在
    if (!config?.cookie || config.cookie === '') {
      toast.error('请先配置Cookie');
      return;
    }

    // 如果正在刷新，则忽略
    if (isRefreshing) {
      return;
    }

    setIsRefreshing(true);
    const loadingToastId = toast.loading('正在刷新数据...');

    try {
      // 使用统一刷新接口，一次请求同时刷新使用数据和积分余额
      await fetch('/api/refresh', { method: 'POST' });
      
      // 刷新后的数据会通过SSE自动推送，无需额外HTTP请求
      toast.success('数据刷新成功', { id: loadingToastId });
    } catch (error) {
      console.error('手动刷新失败:', error);
      toast.error('刷新失败，请稍后重试', { id: loadingToastId });
    } finally {
      setIsRefreshing(false);
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
            {!isConnected ? '连接中...' : lastUpdate ? `最后更新: ${lastUpdate.toLocaleTimeString()}` : '请启用监控或手动刷新获取数据'}
          </p>
        </div>

        {/* 右侧控制区 */}
        <div className="flex items-center space-x-4">
          {/* 积分余额显示 */}
          {creditBalance && (
            <div className="text-white bg-white/10 px-3 py-1 rounded-lg backdrop-blur-sm">
              <div className="text-xs text-white/70">可用积分</div>
              <div className="text-sm font-mono font-bold text-yellow-400">
                {creditBalance.remaining.toLocaleString()}
              </div>
            </div>
          )}
          {/* 连接状态指示 */}
          <div className="flex items-center" title={isConnected ? "已连接" : "连接断开"}>
            {isConnected ? (
              <Wifi className="w-5 h-5 text-green-400" />
            ) : (
              <WifiOff className="w-5 h-5 text-red-400" />
            )}
          </div>

          {/* 监控状态开关 */}
          <div className="flex items-center space-x-2">
            <span className="text-sm text-white/80 hidden md:block">监控</span>
            <button
              onClick={toggleMonitoring}
              disabled={!config?.cookie || config.cookie === '' || !isConnected}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-white/20 disabled:opacity-50 ${
                isMonitoring ? 'bg-green-500' : 'bg-gray-600'
              }`}
              title={!isConnected ? "请等待连接建立" : (!config?.cookie || config.cookie === '') ? "请先配置Cookie" : "切换监控状态"}
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
            disabled={(!config?.cookie || config.cookie === '') || isRefreshing || !isConnected}
            className="p-2 text-white/80 hover:text-white hover:bg-white/10 rounded-lg transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            title={!isConnected ? "请等待连接建立" : (!config?.cookie || config.cookie === '') ? "请先配置Cookie" : "手动刷新数据"}
          >
            <RefreshCw className={`w-5 h-5 ${isRefreshing ? 'animate-spin' : ''}`} />
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
          {usageData.length === 0 ? (
            <div className="h-full flex items-center justify-center">
              <div className="text-center text-white/60">
                <div className="text-6xl mb-4">📊</div>
                <h2 className="text-xl mb-2">暂无数据</h2>
                <p className="text-sm">
                  {!isConnected ? '请等待连接建立' : '请启用监控或点击刷新按钮获取数据'}
                </p>
              </div>
            </div>
          ) : (
            <UsageChart data={usageData} className="h-full" />
          )}
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