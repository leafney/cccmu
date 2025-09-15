import { useState, useEffect } from 'react';
import { Save, Settings, Trash2, Clock } from 'lucide-react';
import type { IUserConfig, IUserConfigRequest } from '../types';
import { apiClient } from '../api/client';

interface SettingsPanelProps {
  className?: string;
  onConfigUpdate?: (config: IUserConfig) => void;
}

export function SettingsPanel({ className = '', onConfigUpdate }: SettingsPanelProps) {
  const [config, setConfig] = useState<IUserConfig>({
    cookie: false,
    interval: 1,
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
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      const response = await apiClient.getConfig();
      if (response.data) {
        setConfig(response.data);
        // 如果已配置cookie，显示占位符，否则清空输入框
        setCookieInput(response.data.cookie ? '' : '');
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
        autoSchedule: config.autoSchedule
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
    setConfig(prev => ({ ...prev, [field]: value }));
  };

  return (
    <div className={`@container bg-white rounded-xl shadow-sm border border-gray-200/60 p-4 @sm:p-6 ${className}`}>
      <div className="flex items-center mb-4 @sm:mb-6">
        <Settings className="w-5 h-5 mr-2 text-gray-600" />
        <h2 className="text-base font-semibold text-gray-900 @sm:text-lg">监控设置</h2>
      </div>

      <div className="space-y-4 @sm:space-y-5">
        {/* Cookie配置 */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Claude Cookie
          </label>
          <div className="flex gap-2">
            <input
              type="password"
              placeholder={config.cookie ? "Cookie已设置，留空不修改" : "请输入Claude网站的Cookie"}
              className="flex-1 px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 hover:border-gray-400"
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
            从浏览器开发者工具中复制Claude网站的Cookie
          </p>
        </div>

        {/* 监控间隔 */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            监控间隔 (分钟)
          </label>
          <select
            className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 hover:border-gray-400 bg-white"
            value={config.interval}
            onChange={(e) => updateConfig('interval', parseInt(e.target.value))}
          >
            <option value={1}>1分钟</option>
            <option value={5}>5分钟</option>
            <option value={10}>10分钟</option>
            <option value={30}>30分钟</option>
            <option value={60}>1小时</option>
          </select>
        </div>

        {/* 显示时间范围 */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            显示时间范围
          </label>
          <select
            className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 hover:border-gray-400 bg-white"
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
            <h3 className="text-sm font-semibold text-gray-900">自动调度</h3>
          </div>
          
          {/* 启用自动调度开关 */}
          <div className="mb-4">
            <label className="flex items-center">
              <input
                type="checkbox"
                checked={config.autoSchedule.enabled}
                onChange={(e) => updateConfig('autoSchedule', { 
                  ...config.autoSchedule, 
                  enabled: e.target.checked 
                })}
                className="w-4 h-4 text-blue-600 border-gray-300 rounded focus:ring-2 focus:ring-blue-500/20"
              />
              <span className="ml-2 text-sm text-gray-700">启用自动调度</span>
            </label>
            <p className="text-xs text-gray-500 mt-1 ml-6">
              启用后监控开关将由系统自动控制，您无法手动操作
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
                    每日 {config.autoSchedule.startTime} - {config.autoSchedule.endTime} 
                    {config.autoSchedule.monitoringOn ? '开启' : '关闭'}监控
                  </p>
                </div>
              )}
            </div>
          )}
        </div>

        {/* 操作按钮 - 改进的响应式布局 */}
        <div className="space-y-3 pt-4 @sm:pt-5">
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

        {/* 状态消息 - 优化视觉设计 */}
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
      </div>
    </div>
  );
}