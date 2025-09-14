import { useState, useEffect, useCallback, useRef } from 'react';
import { UsageChart } from '../components/UsageChart';
import { SettingsModal } from '../components/SettingsModal';
import type { IUsageData, IUserConfig, IUserConfigRequest, ICreditBalance, IMonitoringStatus } from '../types';
import { apiClient } from '../api/client';
import { Settings, Wifi, WifiOff, RefreshCw, BarChart3, X } from 'lucide-react';
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
  const [showConfirmDialog, setShowConfirmDialog] = useState(false);
  const [isAutoResetEnabled, setIsAutoResetEnabled] = useState(false);
  const [monitoringStatus, setMonitoringStatus] = useState<IMonitoringStatus | null>(null);
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
      (resetUsed: boolean) => {
        // 处理重置状态更新
        console.debug('收到重置状态更新:', resetUsed);
        setConfig(prev => prev ? { ...prev, dailyResetUsed: resetUsed } : prev);
      },
      (status: IMonitoringStatus) => {
        // 处理监控状态更新
        console.debug('收到监控状态更新:', status);
        setMonitoringStatus(status);
        setIsMonitoring(status.isMonitoring);
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
          const loadedConfig = configResponse.data;
          
          // 应用互控逻辑：如果自动调度已启用，确保监控开关也启用
          const adjustedConfig = {
            ...loadedConfig,
            enabled: loadedConfig.autoSchedule.enabled ? true : loadedConfig.enabled
          };
          
          setConfig(adjustedConfig);
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
    // 如果启用了自动调度，禁止手动操作
    if (monitoringStatus?.autoScheduleEnabled) {
      toast.error('自动调度已启用，请在设置中关闭自动调度后再手动操作');
      return;
    }
    try {
      if (isMonitoring) {
        // 先立即更新UI状态为关闭，提供即时反馈
        setIsMonitoring(false);
        
        try {
          try {
            await apiClient.stopTask();
            toast.success('已关闭动态监控，请手动刷新获取最新数据');
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
            const requestConfig: IUserConfigRequest = {
              interval: config.interval,
              timeRange: config.timeRange,
              enabled: false
            };
            try {
              await apiClient.updateConfig(requestConfig);
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
            toast.success('已开启动态监控，将自动获取最新数据');
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
            const requestConfig: IUserConfigRequest = {
              interval: config.interval,
              timeRange: config.timeRange,
              enabled: true
            };
            try {
              await apiClient.updateConfig(requestConfig);
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

    // 检查Cookie是否已配置
    if (!config?.cookie) {
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

  // 重置积分
  const handleResetCredits = () => {
    // 检查Cookie是否已配置
    if (!config?.cookie) {
      toast.error('请先配置Cookie');
      return;
    }

    // 显示确认弹窗
    setShowConfirmDialog(true);
  };

  // 确认重置积分
  const confirmResetCredits = async () => {
    setShowConfirmDialog(false);
    
    const loadingToastId = toast.loading('正在重置积分...');

    try {
      await apiClient.resetCredits();
      toast.success('积分重置成功', { id: loadingToastId });
      // 数据刷新由后端处理，前端通过SSE自动接收最新数据
    } catch (error) {
      console.error('重置积分失败:', error);
      toast.error(error instanceof Error ? error.message : '重置失败，请稍后重试', { id: loadingToastId });
    }
  };

  // 取消重置积分
  const cancelResetCredits = () => {
    setShowConfirmDialog(false);
  };

  // 切换自动重置状态
  const toggleAutoReset = async () => {
    try {
      // TODO: 实现自动重置API调用
      setIsAutoResetEnabled(!isAutoResetEnabled);
      // 这里将来需要调用后端API来设置自动重置状态
      toast.success(isAutoResetEnabled ? '已关闭每日积分自动重置功能' : '已开启每日积分自动重置功能');
    } catch (error) {
      console.error('切换自动重置状态失败:', error);
      toast.error('操作失败，请稍后重试');
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-blue-900 to-purple-900">
      {/* 顶部控制栏 */}
      <div className="absolute top-0 left-0 right-0 z-10 flex items-center justify-between p-4 md:p-6">
        {/* 左侧标题和连接状态 */}
        <div className="text-white">
          <div className="flex items-center space-x-3">
            <h1 className="text-xl md:text-2xl font-bold">Claude 积分监控</h1>
            {/* 连接状态指示 */}
            <div className="flex items-center" title={isConnected ? "已连接" : "连接断开"}>
              {isConnected ? (
                <Wifi className="w-5 h-5 text-green-400" />
              ) : (
                <WifiOff className="w-5 h-5 text-red-400" />
              )}
            </div>
          </div>
          <p className="text-sm text-white/70 mt-1">
            {!config?.cookie 
              ? '请先配置Cookie' 
              : !isConnected 
                ? '连接中...' 
                : lastUpdate 
                  ? `最后更新: ${lastUpdate.toLocaleTimeString()}` 
                  : '请启用监控或手动刷新获取数据'
            }
          </p>
        </div>

        {/* 右侧控制区 */}
        <div className="flex items-center space-x-4">
          {/* 积分余额显示 - 可点击重置 */}
          {creditBalance && (
            <button
              onClick={handleResetCredits}
              disabled={!config?.cookie || !isConnected}
              className={`text-white bg-white/10 px-3 py-1 rounded-lg backdrop-blur-sm hover:bg-white/20 transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:bg-white/10 ${
                isAutoResetEnabled
                  ? 'shadow-[0_0_15px_rgba(168,85,247,0.5)] border border-purple-500/30' // 自动重置：紫色光晕
                  : config?.dailyResetUsed 
                    ? 'shadow-[0_0_15px_rgba(239,68,68,0.5)] border border-red-500/30' // 已重置：红色光晕
                    : 'shadow-[0_0_15px_rgba(34,197,94,0.5)] border border-green-500/30' // 未重置：绿色光晕
              }`}
              title={
                isAutoResetEnabled
                  ? "每日自动重置已启用"
                  : config?.dailyResetUsed 
                    ? "今日已重置过积分" 
                    : "点击重置积分（今日可重置）"
              }
            >
              <div className="text-xs text-white/70">可用积分</div>
              <div className="text-sm font-mono font-bold text-yellow-400">
                {creditBalance.remaining.toLocaleString()}
              </div>
            </button>
          )}

          {/* 开关组 - 竖向排列 */}
          <div className="flex flex-col space-y-2">
            {/* 监控状态开关 */}
            <div className="flex items-center space-x-2">
              <span className="text-xs text-white/80 hidden md:block">监控</span>
              <button
                onClick={toggleMonitoring}
                disabled={!config?.cookie || !isConnected || monitoringStatus?.autoScheduleEnabled}
                className={`relative inline-flex h-4 w-8 items-center rounded-full transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-white/20 disabled:opacity-50 ${
                  isMonitoring ? 'bg-green-500' : 'bg-gray-600'
                } ${
                  monitoringStatus?.autoScheduleEnabled 
                    ? 'ring-2 ring-orange-400/75 shadow-lg shadow-orange-400/25 animate-pulse' 
                    : ''
                }`}
                title={
                  monitoringStatus?.autoScheduleEnabled 
                    ? "自动调度已启用，无法手动操作" 
                    : !isConnected 
                    ? "请等待连接建立" 
                    : !config?.cookie 
                    ? "请先配置Cookie" 
                    : "切换监控状态"
                }
              >
                <span
                  className={`inline-block h-3 w-3 transform rounded-full bg-white transition ${
                    isMonitoring ? 'translate-x-4' : 'translate-x-0.5'
                  }`}
                />
              </button>
            </div>
            
            {/* 自动重置开关 */}
            <div className="flex items-center space-x-2">
              <span className="text-xs text-white/80 hidden md:block">重置</span>
              <button
                onClick={toggleAutoReset}
                disabled={!config?.cookie || !isConnected}
                className={`relative inline-flex h-4 w-8 items-center rounded-full transition-colors focus:outline-none focus:ring-2 focus:ring-white/20 disabled:opacity-50 ${
                  isAutoResetEnabled ? 'bg-purple-500' : 'bg-gray-600'
                }`}
                title={!isConnected ? "请等待连接建立" : !config?.cookie ? "请先配置Cookie" : "切换自动重置状态"}
              >
                <span
                  className={`inline-block h-3 w-3 transform rounded-full bg-white transition ${
                    isAutoResetEnabled ? 'translate-x-4' : 'translate-x-0.5'
                  }`}
                />
              </button>
            </div>
          </div>

          {/* 图标按钮组 - 竖向排列 */}
          <div className="flex flex-col space-y-1.5">
            {/* 手动刷新按钮 */}
            <button
              onClick={handleRefresh}
              disabled={!config?.cookie || isRefreshing || !isConnected}
              className="p-1 text-white/80 hover:text-white hover:bg-white/10 rounded-md transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
              title={!isConnected ? "请等待连接建立" : !config?.cookie ? "请先配置Cookie" : "手动刷新数据"}
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
            </button>

            {/* 设置按钮 */}
            <button
              onClick={() => setShowSettings(true)}
              className="p-1 text-white/80 hover:text-white hover:bg-white/10 rounded-md transition-colors"
              title="打开设置"
            >
              <Settings className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* 全屏图表区域 */}
      <div className="h-screen w-full flex items-center justify-center p-4 pt-20">
        <div className="w-full h-full max-h-[calc(100vh-100px)]">
          {usageData.length === 0 ? (
            <div className="h-full flex items-center justify-center">
              <div className="text-center text-white/60">
                <div className="mb-6">
                  <BarChart3 className="w-16 h-16 mx-auto text-white/40" />
                </div>
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
        isMonitoring={isMonitoring}
        monitoringStatus={monitoringStatus}
      />

      {/* 重置积分确认弹窗 */}
      {showConfirmDialog && (
        <div className="fixed inset-0 z-50 overflow-y-auto">
          {/* 背景遮罩 */}
          <div 
            className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
            onClick={cancelResetCredits}
          />
          
          {/* 弹窗内容 */}
          <div className="flex min-h-full items-center justify-center p-4">
            <div className="relative bg-white rounded-xl shadow-xl w-full max-w-md p-6 transform transition-all">
              {/* 关闭按钮 */}
              <button
                onClick={cancelResetCredits}
                className="absolute top-4 right-4 p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
              >
                <X className="w-5 h-5" />
              </button>

              {/* 标题 */}
              <h3 className="text-lg font-semibold text-gray-900 mb-4">
                重置积分
              </h3>

              {/* 内容 */}
              <p className="text-gray-600 mb-6">
                确认要重置积分吗？每日仅有一次重置机会。
              </p>

              {/* 按钮组 */}
              <div className="flex space-x-3 justify-end">
                <button
                  onClick={cancelResetCredits}
                  className="px-4 py-2 text-sm font-medium text-gray-700 bg-gray-100 hover:bg-gray-200 rounded-lg transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={confirmResetCredits}
                  className="px-4 py-2 text-sm font-medium text-white bg-red-600 hover:bg-red-700 rounded-lg transition-colors"
                >
                  确认重置
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}