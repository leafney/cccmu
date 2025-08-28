import { useState, useEffect } from 'react';
import { Save, Settings, Trash2 } from 'lucide-react';
import type { IUserConfig } from '../types';
import { apiClient } from '../api/client';

interface SettingsPanelProps {
  className?: string;
  onConfigUpdate?: (config: IUserConfig) => void;
}

export function SettingsPanel({ className = '', onConfigUpdate }: SettingsPanelProps) {
  const [config, setConfig] = useState<IUserConfig>({
    cookie: '',
    interval: 1,
    timeRange: 60,
    enabled: false
  });
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
      }
    } catch (error) {
      console.error('加载配置失败:', error);
      showMessage('error', '加载配置失败');
    }
  };

  const handleSaveConfig = async () => {
    try {
      setSaving(true);
      await apiClient.updateConfig(config);
      showMessage('success', '配置保存成功');
      onConfigUpdate?.(config);
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
      const updatedConfig = { ...config, cookie: '' };
      setConfig(updatedConfig);
      
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
              placeholder="请输入Claude网站的Cookie"
              className="flex-1 px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-500 transition-all duration-200 hover:border-gray-400"
              value={config.cookie === '已设置' ? '' : config.cookie}
              onChange={(e) => updateConfig('cookie', e.target.value)}
            />
            {config.cookie && config.cookie !== '' && (
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
          {config.cookie === '已设置' && (
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