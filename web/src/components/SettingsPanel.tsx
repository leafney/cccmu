import { useState, useEffect } from 'react';
import { Save, Settings, Play, Square } from 'lucide-react';
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
    timeRange: 1,
    enabled: false
  });
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [taskRunning, setTaskRunning] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // 加载当前配置
  useEffect(() => {
    loadConfig();
    checkTaskStatus();
  }, []);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const response = await apiClient.getConfig();
      if (response.data) {
        setConfig(response.data);
      }
    } catch (error) {
      console.error('加载配置失败:', error);
      showMessage('error', '加载配置失败');
    } finally {
      setLoading(false);
    }
  };

  const checkTaskStatus = async () => {
    try {
      await apiClient.getConfig(); // 可以添加专门的状态检查接口
      // 这里可以检查任务运行状态
    } catch (error) {
      console.error('检查任务状态失败:', error);
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

  const handleStartTask = async () => {
    try {
      setLoading(true);
      await apiClient.startTask();
      setTaskRunning(true);
      showMessage('success', '监控任务已启动');
    } catch (error) {
      console.error('启动任务失败:', error);
      showMessage('error', '启动任务失败');
    } finally {
      setLoading(false);
    }
  };

  const handleStopTask = async () => {
    try {
      setLoading(true);
      await apiClient.stopTask();
      setTaskRunning(false);
      showMessage('success', '监控任务已停止');
    } catch (error) {
      console.error('停止任务失败:', error);
      showMessage('error', '停止任务失败');
    } finally {
      setLoading(false);
    }
  };

  const handleRefreshData = async () => {
    try {
      setLoading(true);
      await apiClient.refreshData();
      showMessage('success', '数据刷新成功');
    } catch (error) {
      console.error('刷新数据失败:', error);
      showMessage('error', '刷新数据失败');
    } finally {
      setLoading(false);
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
    <div className={`bg-white rounded-lg shadow-sm border p-6 ${className}`}>
      <div className="flex items-center mb-6">
        <Settings className="w-5 h-5 mr-2 text-gray-600" />
        <h2 className="text-lg font-semibold text-gray-900">监控设置</h2>
      </div>

      <div className="space-y-4">
        {/* Cookie配置 */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            Claude Cookie
          </label>
          <input
            type="password"
            placeholder="请输入Claude网站的Cookie"
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            value={config.cookie === '已设置' ? '' : config.cookie}
            onChange={(e) => updateConfig('cookie', e.target.value)}
          />
          {config.cookie === '已设置' && (
            <p className="text-sm text-green-600 mt-1">Cookie已设置</p>
          )}
          <p className="text-xs text-gray-500 mt-1">
            从浏览器开发者工具中复制Claude网站的Cookie
          </p>
        </div>

        {/* 监控间隔 */}
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-2">
            监控间隔 (分钟)
          </label>
          <select
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
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
            显示时间范围 (小时)
          </label>
          <select
            className="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
            value={config.timeRange}
            onChange={(e) => updateConfig('timeRange', parseInt(e.target.value))}
          >
            <option value={1}>1小时</option>
            <option value={6}>6小时</option>
            <option value={12}>12小时</option>
            <option value={24}>24小时</option>
          </select>
        </div>

        {/* 操作按钮 */}
        <div className="flex gap-3 pt-4">
          <button
            onClick={handleSaveConfig}
            disabled={saving || loading}
            className="flex-1 flex items-center justify-center px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            <Save className="w-4 h-4 mr-2" />
            {saving ? '保存中...' : '保存配置'}
          </button>

          {taskRunning ? (
            <button
              onClick={handleStopTask}
              disabled={loading}
              className="flex items-center justify-center px-4 py-2 bg-red-600 text-white rounded-md hover:bg-red-700 disabled:opacity-50"
            >
              <Square className="w-4 h-4 mr-2" />
              {loading ? '停止中...' : '停止'}
            </button>
          ) : (
            <button
              onClick={handleStartTask}
              disabled={loading || !config.cookie || config.cookie === ''}
              className="flex items-center justify-center px-4 py-2 bg-green-600 text-white rounded-md hover:bg-green-700 disabled:opacity-50"
            >
              <Play className="w-4 h-4 mr-2" />
              {loading ? '启动中...' : '启动'}
            </button>
          )}

          <button
            onClick={handleRefreshData}
            disabled={loading}
            className="px-4 py-2 bg-gray-600 text-white rounded-md hover:bg-gray-700 disabled:opacity-50"
          >
            {loading ? '刷新中...' : '手动刷新'}
          </button>
        </div>

        {/* 状态消息 */}
        {message && (
          <div className={`mt-4 p-3 rounded-md ${
            message.type === 'success' ? 'bg-green-50 text-green-800' : 'bg-red-50 text-red-800'
          }`}>
            {message.text}
          </div>
        )}
      </div>
    </div>
  );
}