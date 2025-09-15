import { useState, useEffect } from 'react';
import { Save, Settings, X, Trash2, Clock, HelpCircle } from 'lucide-react';
import type { IUserConfig, IUserConfigRequest, IMonitoringStatus } from '../types';
import { apiClient } from '../api/client';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  onConfigUpdate?: (config: IUserConfig) => void;
  isMonitoring?: boolean;
  monitoringStatus?: IMonitoringStatus | null;
}

export function SettingsModal({ isOpen, onClose, onConfigUpdate, isMonitoring = false }: SettingsModalProps) {
  const [config, setConfig] = useState<IUserConfig>({
    cookie: false,
    interval: 60,
    timeRange: 60,
    enabled: false,
    dailyResetUsed: false,
    autoSchedule: {
      enabled: false,
      startTime: '',
      endTime: '',
      monitoringOn: true
    },
    autoReset: {
      enabled: false,
      timeEnabled: false,
      resetTime: '',
      thresholdEnabled: false,
      threshold: 0
    }
  });
  const [cookieInput, setCookieInput] = useState<string>('');
  const [saving, setSaving] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // 加载当前配置
  useEffect(() => {
    if (isOpen) {
      loadConfig();
    }
  }, [isOpen]);

  const loadConfig = async () => {
    try {
      const response = await apiClient.getConfig();
      if (response.data) {
        const loadedConfig = response.data;
        
        // 应用互控逻辑：如果自动调度已启用，确保监控开关也启用
        const adjustedConfig = {
          ...loadedConfig,
          enabled: loadedConfig.autoSchedule.enabled ? true : loadedConfig.enabled
        };
        
        setConfig(adjustedConfig);
        // 如果已配置cookie，显示占位符，否则清空输入框
        setCookieInput(loadedConfig.cookie ? '' : '');
      }
    } catch (error) {
      console.error('加载配置失败:', error);
      showMessage('error', '加载配置失败');
    }
  };

  const handleSaveConfig = async () => {
    try {
      setSaving(true);
      // 构建请求配置，包含真实的cookie字符串
      const requestConfig: IUserConfigRequest = {
        interval: config.interval,
        timeRange: config.timeRange,
        enabled: config.enabled,
        autoSchedule: config.autoSchedule,
        autoReset: config.autoReset
      };
      
      // 只有在输入了新的Cookie时才发送Cookie字段
      if (cookieInput && cookieInput.trim() !== '') {
        requestConfig.cookie = cookieInput.trim();
      }
      
      await apiClient.updateConfig(requestConfig);
      
      // 更新本地状态为boolean值
      const updatedConfig = {
        ...config,
        cookie: (cookieInput && cookieInput.trim() !== '') ? true : config.cookie // 只有输入了新Cookie才更新状态
      };
      setConfig(updatedConfig);
      
      showMessage('success', '配置保存成功');
      onConfigUpdate?.(updatedConfig);
    } catch (error) {
      console.error('保存配置失败:', error);
      showMessage('error', '保存配置失败');
    } finally {
      setSaving(false);
    }
  };

  const handleClearCookie = async () => {
    try {
      setClearing(true);
      await apiClient.clearCookie();
      
      // 更新本地状态
      const updatedConfig = { ...config, cookie: false };
      setConfig(updatedConfig);
      setCookieInput('');
      
      showMessage('success', 'Cookie已清除，监控已停止');
      onConfigUpdate?.(updatedConfig);
    } catch (error) {
      console.error('清除Cookie失败:', error);
      showMessage('error', '清除Cookie失败');
    } finally {
      setClearing(false);
    }
  };


  const showMessage = (type: 'success' | 'error', text: string) => {
    setMessage({ type, text });
    setTimeout(() => setMessage(null), 3000);
  };

  const updateConfig = (field: keyof IUserConfig, value: any) => {
    setConfig(prev => {
      const newConfig = { ...prev, [field]: value };
      
      // 特殊处理：如果启用自动调度，自动启用监控配置
      if (field === 'autoSchedule' && value.enabled) {
        newConfig.enabled = true;
      }
      
      return newConfig;
    });
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* 背景遮罩 */}
      <div 
        className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
        onClick={onClose}
      />
      
      {/* 模态框 */}
      <div className="flex min-h-full items-center justify-center p-4">
        <div className="relative bg-white rounded-2xl shadow-xl w-full max-w-md p-6 transform transition-all">
          {/* 标题栏 */}
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center">
              <Settings className="w-5 h-5 mr-2 text-gray-600" />
              <h2 className="text-lg font-semibold text-gray-900">监控设置</h2>
            </div>
            <button
              onClick={onClose}
              className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          <div className="space-y-5">
            {/* Cookie配置 */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                ACM Cookie
              </label>
              <div className="flex gap-2">
                <input
                  type="password"
                  placeholder={config.cookie ? "Cookie已设置，留空不修改" : "请输入 ACM 网站的Cookie"}
                  className="flex-1 px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200"
                  value={cookieInput}
                  onChange={(e) => setCookieInput(e.target.value)}
                />
                {config.cookie && (
                  <button
                    onClick={handleClearCookie}
                    disabled={clearing}
                    className="px-3 py-2.5 bg-red-600 text-white text-sm font-medium rounded-lg hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500/20 disabled:opacity-50 transition-all duration-200 flex items-center"
                    title="清除Cookie"
                  >
                    <Trash2 className="w-4 h-4" />
                    {clearing && <span className="ml-1">...</span>}
                  </button>
                )}
              </div>
              {config.cookie && (
                <p className="text-sm text-green-600 mt-1.5 flex items-center">
                  <svg className="w-4 h-4 mr-1.5" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                  </svg>
                  Cookie已设置
                </p>
              )}
              <p className="text-xs text-gray-500 mt-1.5 leading-relaxed">
                从浏览器开发者工具中复制 ACM 网站的Cookie
              </p>
            </div>

            {/* 监控间隔 */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                监控间隔
              </label>
              <select
                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 bg-white"
                value={config.interval}
                onChange={(e) => updateConfig('interval', parseInt(e.target.value))}
              >
                <option value={30}>30秒</option>
                <option value={60}>1分钟</option>
                <option value={300}>5分钟</option>
                <option value={600}>10分钟</option>
                <option value={1800}>30分钟</option>
                <option value={3600}>1小时</option>
              </select>
            </div>

            {/* 显示时间范围 */}
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">
                显示时间范围
              </label>
              <select
                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 bg-white"
                value={config.timeRange}
                onChange={(e) => updateConfig('timeRange', parseInt(e.target.value))}
              >
                <option value={30}>30分钟</option>
                <option value={60}>1小时</option>
                <option value={120}>2小时</option>
                <option value={180}>3小时</option>
                <option value={300}>5小时</option>
                <option value={360}>6小时</option>
                <option value={720}>12小时</option>
                <option value={1440}>24小时</option>
              </select>
            </div>

            {/* 自动调度配置 */}
            <div className="pt-4 border-t border-gray-200">
              <div className="flex items-center mb-4">
                <Clock className="w-5 h-5 mr-2 text-gray-600" />
                <h3 className="text-sm font-semibold text-gray-900">动态监控自动调度</h3>
                <div className="relative group">
                  <HelpCircle className="w-4 h-4 ml-2 text-gray-400 cursor-help" />
                  <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-64 p-2 bg-gray-800 text-white text-xs rounded-lg shadow-lg opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none z-50">
                    自动调度功能需要先开启监控开关才能使用。启用后系统将根据设定的时间自动控制监控的开启和关闭。
                  </div>
                </div>
              </div>
              
              {/* 启用自动调度开关 */}
              <div className="mb-4">
                <div className="flex items-center justify-between">
                  <span className={`text-sm font-medium ${isMonitoring ? 'text-gray-700' : 'text-gray-400'}`}>
                    启用动态监控自动调度
                  </span>
                  <button
                    type="button"
                    disabled={!isMonitoring}
                    onClick={() => updateConfig('autoSchedule', { 
                      ...config.autoSchedule, 
                      enabled: !config.autoSchedule.enabled 
                    })}
                    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-blue-500/20 disabled:opacity-50 disabled:cursor-not-allowed ${
                      config.autoSchedule.enabled ? 'bg-blue-600' : 'bg-gray-300'
                    }`}
                  >
                    <span
                      className={`inline-block h-3 w-3 transform rounded-full bg-white transition duration-200 ${
                        config.autoSchedule.enabled ? 'translate-x-5' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-2">
                  {!isMonitoring
                    ? '请先开启监控开关才能使用此功能' 
                    : '启用后监控开关将由系统自动控制，您无法手动操作'
                  }
                </p>
              </div>

              {/* 自动调度详细配置 */}
              {config.autoSchedule.enabled && (
                <div className="space-y-4 ml-6 p-4 bg-gray-50 rounded-lg border border-gray-200">
                  {/* 时间范围设置 */}
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        开始时间
                      </label>
                      <input
                        type="time"
                        value={config.autoSchedule.startTime}
                        onChange={(e) => updateConfig('autoSchedule', { 
                          ...config.autoSchedule, 
                          startTime: e.target.value 
                        })}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        结束时间
                      </label>
                      <input
                        type="time"
                        value={config.autoSchedule.endTime}
                        onChange={(e) => updateConfig('autoSchedule', { 
                          ...config.autoSchedule, 
                          endTime: e.target.value 
                        })}
                        className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500"
                      />
                    </div>
                  </div>

                  {/* 监控状态设置 */}
                  <div>
                    <label className="block text-sm font-medium text-gray-700 mb-2">
                      时间范围内的监控状态
                    </label>
                    <div className="space-y-2">
                      <label className="flex items-center">
                        <input
                          type="radio"
                          name="monitoringOn"
                          checked={config.autoSchedule.monitoringOn === true}
                          onChange={() => updateConfig('autoSchedule', { 
                            ...config.autoSchedule, 
                            monitoringOn: true 
                          })}
                          className="w-4 h-4 text-blue-600 border-gray-300 focus:ring-2 focus:ring-blue-500/20"
                        />
                        <span className="ml-2 text-sm text-gray-700">时间范围内开启监控</span>
                      </label>
                      <label className="flex items-center">
                        <input
                          type="radio"
                          name="monitoringOn"
                          checked={config.autoSchedule.monitoringOn === false}
                          onChange={() => updateConfig('autoSchedule', { 
                            ...config.autoSchedule, 
                            monitoringOn: false 
                          })}
                          className="w-4 h-4 text-blue-600 border-gray-300 focus:ring-2 focus:ring-blue-500/20"
                        />
                        <span className="ml-2 text-sm text-gray-700">时间范围内关闭监控</span>
                      </label>
                    </div>
                    <p className="text-xs text-gray-500 mt-2">
                      例如：设置22:00-06:00并选择"关闭监控"，则在晚10点到次日早6点期间自动关闭监控
                    </p>
                  </div>

                  {/* 当前状态预览 */}
                  {config.autoSchedule.startTime && config.autoSchedule.endTime && (
                    <div className="mt-3 p-3 bg-blue-50 rounded-lg border border-blue-200">
                      <p className="text-sm text-blue-800">
                        <strong>配置预览：</strong>
                        每日 {config.autoSchedule.startTime} - {
                          config.autoSchedule.startTime > config.autoSchedule.endTime 
                            ? `次日 ${config.autoSchedule.endTime}` 
                            : config.autoSchedule.endTime
                        } {config.autoSchedule.monitoringOn ? '开启' : '关闭'}监控
                      </p>
                    </div>
                  )}
                </div>
              )}
            </div>

            {/* 自动重置配置 */}
            <div className="pt-4 border-t border-gray-200">
              <div className="flex items-center mb-4">
                <Clock className="w-5 h-5 mr-2 text-gray-600" />
                <h3 className="text-sm font-semibold text-gray-900">积分自动重置</h3>
                <div className="relative group">
                  <HelpCircle className="w-4 h-4 ml-2 text-gray-400 cursor-help" />
                  <div className="absolute bottom-full left-1/2 transform -translate-x-1/2 mb-2 w-64 p-2 bg-gray-800 text-white text-xs rounded-lg shadow-lg opacity-0 group-hover:opacity-100 transition-opacity duration-200 pointer-events-none z-50">
                    启用后系统将根据设定的时间自动执行积分重置操作，每天最多自动重置一次。
                  </div>
                </div>
              </div>
              
              {/* 启用自动重置开关 */}
              <div className="mb-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-medium text-gray-700">
                    启用积分自动重置
                  </span>
                  <button
                    type="button"
                    onClick={async () => {
                      const newAutoResetConfig = { 
                        ...config.autoReset, 
                        enabled: !config.autoReset.enabled 
                      };
                      
                      // 立即更新本地状态
                      updateConfig('autoReset', newAutoResetConfig);
                      
                      // 立即保存到后端
                      try {
                        const requestConfig: IUserConfigRequest = {
                          interval: config.interval,
                          timeRange: config.timeRange,
                          enabled: config.enabled,
                          autoSchedule: config.autoSchedule,
                          autoReset: newAutoResetConfig
                        };
                        
                        await apiClient.updateConfig(requestConfig);
                        
                        // 通知父组件更新
                        const updatedConfig = {
                          ...config,
                          autoReset: newAutoResetConfig
                        };
                        onConfigUpdate?.(updatedConfig);
                        
                        showMessage('success', newAutoResetConfig.enabled ? '已开启自动重置功能' : '已关闭自动重置功能');
                      } catch (error) {
                        console.error('更新自动重置配置失败:', error);
                        // 回滚本地状态
                        updateConfig('autoReset', config.autoReset);
                        showMessage('error', '更新配置失败');
                      }
                    }}
                    className={`relative inline-flex h-5 w-9 items-center rounded-full transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 ${
                      config.autoReset.enabled ? 'bg-purple-600' : 'bg-gray-300'
                    }`}
                  >
                    <span
                      className={`inline-block h-3 w-3 transform rounded-full bg-white transition duration-200 ${
                        config.autoReset.enabled ? 'translate-x-5' : 'translate-x-1'
                      }`}
                    />
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-2">
                  启用后将根据设定时间自动重置积分，每天最多重置一次
                </p>
              </div>

              {/* 自动重置详细配置 */}
              {config.autoReset.enabled && (
                <div className="space-y-4 ml-6 p-4 bg-purple-50 rounded-lg border border-purple-200">
                  {/* 重置时间设置 */}
                  <div>
                    <div className="flex items-center justify-between mb-2">
                      <label className="text-sm font-medium text-gray-700">
                        重置时间
                      </label>
                      <button
                        type="button"
                        onClick={() => updateConfig('autoReset', { 
                          ...config.autoReset, 
                          timeEnabled: !config.autoReset.timeEnabled 
                        })}
                        className={`relative inline-flex h-4 w-7 items-center rounded-full transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-purple-500/20 ${
                          config.autoReset.timeEnabled ? 'bg-purple-600' : 'bg-gray-300'
                        }`}
                      >
                        <span
                          className={`inline-block h-3 w-3 transform rounded-full bg-white transition duration-200 ${
                            config.autoReset.timeEnabled ? 'translate-x-3.5' : 'translate-x-0.5'
                          }`}
                        />
                      </button>
                    </div>
                    <input
                      type="time"
                      value={config.autoReset.resetTime}
                      disabled={!config.autoReset.timeEnabled}
                      onChange={(e) => updateConfig('autoReset', { 
                        ...config.autoReset, 
                        resetTime: e.target.value 
                      })}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-purple-500/20 focus:border-purple-500 disabled:opacity-50 disabled:cursor-not-allowed disabled:bg-gray-50"
                    />
                    <p className="text-xs text-gray-500 mt-1">
                      每天在此时间自动执行积分重置操作
                    </p>
                  </div>

                  {/* 当前配置预览 */}
                  {config.autoReset.timeEnabled && config.autoReset.resetTime && (
                    <div className="mt-3 p-3 bg-purple-100 rounded-lg border border-purple-200">
                      <p className="text-sm text-purple-800">
                        <strong>配置预览：</strong>
                        每日 {config.autoReset.resetTime} 自动重置积分
                      </p>
                      <p className="text-xs text-purple-600 mt-1">
                        * 如当日已手动重置过，将跳过自动重置
                      </p>
                    </div>
                  )}
                  
                  {/* 无有效条件提示 */}
                  {config.autoReset.enabled && !config.autoReset.timeEnabled && !config.autoReset.thresholdEnabled && (
                    <div className="mt-3 p-3 bg-yellow-50 rounded-lg border border-yellow-200">
                      <p className="text-sm text-yellow-800">
                        <strong>提示：</strong>
                        未启用任何触发条件，自动重置功能将不会执行
                      </p>
                    </div>
                  )}

                  {/* 预留区域：积分阈值触发（暂时禁用） */}
                  <div className="opacity-50 pointer-events-none">
                    <div className="flex items-center justify-between mb-2">
                      <span className="text-sm font-medium text-gray-700">
                        积分低于阈值时触发（开发中）
                      </span>
                      <button
                        type="button"
                        disabled={true}
                        className="relative inline-flex h-5 w-9 items-center rounded-full bg-gray-300 transition-all duration-200"
                      >
                        <span className="inline-block h-3 w-3 transform rounded-full bg-white transition duration-200 translate-x-1" />
                      </button>
                    </div>
                    <input
                      type="number"
                      placeholder="积分阈值"
                      disabled={true}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm bg-gray-100"
                    />
                    <p className="text-xs text-gray-400 mt-1">
                      此功能将在后续版本中提供
                    </p>
                  </div>
                </div>
              )}
            </div>

            {/* 操作按钮 */}
            <div className="space-y-3 pt-4">
              {/* 保存配置按钮 */}
              <button
                onClick={handleSaveConfig}
                disabled={saving}
                className="w-full flex items-center justify-center px-4 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 focus:outline-none focus:ring-2 focus:ring-blue-500/20 disabled:opacity-50 disabled:cursor-not-allowed transition-all duration-200"
              >
                <Save className="w-4 h-4 mr-2" />
                {saving ? '保存中...' : '保存配置'}
              </button>

            </div>

            {/* 状态消息 */}
            {message && (
              <div className={`mt-4 p-3 rounded-lg border text-sm font-medium transition-all duration-300 ${
                message.type === 'success' 
                  ? 'bg-green-50 text-green-800 border-green-200/50' 
                  : 'bg-red-50 text-red-800 border-red-200/50'
              }`}>
                <div className="flex items-center">
                  {message.type === 'success' ? (
                    <svg className="w-4 h-4 mr-2 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                    </svg>
                  ) : (
                    <svg className="w-4 h-4 mr-2 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                      <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                    </svg>
                  )}
                  {message.text}
                </div>
              </div>
            )}

            {/* A社官方状态链接 */}
            <div className="mt-4 pt-4 border-t border-gray-200 text-center">
              <p className="text-xs text-gray-500">
                A社官方{' '}
                <a 
                  href="https://status.anthropic.com/" 
                  target="_blank" 
                  rel="noopener noreferrer"
                  className="text-blue-600 hover:text-blue-700 hover:underline font-medium transition-colors"
                >
                  Status
                </a>
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}