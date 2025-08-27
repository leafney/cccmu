import { } from 'react';
import { Wifi, WifiOff, Clock, Activity } from 'lucide-react';

interface StatusIndicatorProps {
  isConnected: boolean;
  isMonitoring: boolean;
  lastUpdate?: Date;
  className?: string;
}

export function StatusIndicator({ 
  isConnected, 
  isMonitoring, 
  lastUpdate, 
  className = '' 
}: StatusIndicatorProps) {
  const getConnectionStatus = () => {
    if (isConnected) {
      return {
        icon: <Wifi className="w-4 h-4" />,
        text: '已连接',
        color: 'text-green-700 bg-green-50 border border-green-200/50 hover:bg-green-100'
      };
    } else {
      return {
        icon: <WifiOff className="w-4 h-4" />,
        text: '连接断开',
        color: 'text-red-700 bg-red-50 border border-red-200/50'
      };
    }
  };

  const getMonitoringStatus = () => {
    if (isMonitoring) {
      return {
        icon: <Activity className="w-4 h-4" />,
        text: '监控中',
        color: 'text-blue-700 bg-blue-50 border border-blue-200/50 hover:bg-blue-100'
      };
    } else {
      return {
        icon: <Clock className="w-4 h-4" />,
        text: '已暂停',
        color: 'text-gray-700 bg-gray-50 border border-gray-200/50'
      };
    }
  };

  const connectionStatus = getConnectionStatus();
  const monitoringStatus = getMonitoringStatus();

  return (
    <div className={`@container bg-white rounded-xl shadow-sm border border-gray-200/60 p-4 @sm:p-5 ${className}`}>
      <h3 className="text-sm font-semibold text-gray-900 mb-4 @sm:text-base">系统状态</h3>
      
      <div className="space-y-3 @sm:space-y-4">
        {/* 连接状态 */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-600">连接状态</span>
          <div className={`flex items-center px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 ${connectionStatus.color}`}>
            {connectionStatus.icon}
            <span className="ml-1.5">{connectionStatus.text}</span>
          </div>
        </div>

        {/* 监控状态 */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-600">监控状态</span>
          <div className={`flex items-center px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 ${monitoringStatus.color}`}>
            {monitoringStatus.icon}
            <span className="ml-1.5">{monitoringStatus.text}</span>
          </div>
        </div>

        {/* 最后更新时间 */}
        {lastUpdate && (
          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-600">最后更新</span>
            <span className="text-xs text-gray-500 font-mono">
              {lastUpdate.toLocaleTimeString()}
            </span>
          </div>
        )}

        {/* 连接指示灯 - 优化视觉效果 */}
        <div className="flex items-center justify-between pt-2 border-t border-gray-100">
          <span className="text-sm text-gray-600">实时连接</span>
          <div className="flex items-center">
            <div className={`w-2.5 h-2.5 rounded-full transition-all duration-300 ${
              isConnected ? 'bg-green-500 shadow-sm' : 'bg-red-500'
            } ${isConnected ? 'animate-pulse' : ''}`}></div>
            <span className="ml-2 text-xs text-gray-500">
              {isConnected ? 'SSE连接正常' : 'SSE连接断开'}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}