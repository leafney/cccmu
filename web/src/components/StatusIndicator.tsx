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
        color: 'text-green-600 bg-green-50'
      };
    } else {
      return {
        icon: <WifiOff className="w-4 h-4" />,
        text: '连接断开',
        color: 'text-red-600 bg-red-50'
      };
    }
  };

  const getMonitoringStatus = () => {
    if (isMonitoring) {
      return {
        icon: <Activity className="w-4 h-4" />,
        text: '监控中',
        color: 'text-blue-600 bg-blue-50'
      };
    } else {
      return {
        icon: <Clock className="w-4 h-4" />,
        text: '已暂停',
        color: 'text-gray-600 bg-gray-50'
      };
    }
  };

  const connectionStatus = getConnectionStatus();
  const monitoringStatus = getMonitoringStatus();

  return (
    <div className={`bg-white rounded-lg shadow-sm border p-4 ${className}`}>
      <h3 className="text-sm font-medium text-gray-900 mb-3">系统状态</h3>
      
      <div className="space-y-3">
        {/* 连接状态 */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-600">连接状态</span>
          <div className={`flex items-center px-2 py-1 rounded-full text-xs font-medium ${connectionStatus.color}`}>
            {connectionStatus.icon}
            <span className="ml-1">{connectionStatus.text}</span>
          </div>
        </div>

        {/* 监控状态 */}
        <div className="flex items-center justify-between">
          <span className="text-sm text-gray-600">监控状态</span>
          <div className={`flex items-center px-2 py-1 rounded-full text-xs font-medium ${monitoringStatus.color}`}>
            {monitoringStatus.icon}
            <span className="ml-1">{monitoringStatus.text}</span>
          </div>
        </div>

        {/* 最后更新时间 */}
        {lastUpdate && (
          <div className="flex items-center justify-between">
            <span className="text-sm text-gray-600">最后更新</span>
            <span className="text-xs text-gray-500">
              {lastUpdate.toLocaleTimeString()}
            </span>
          </div>
        )}

        {/* 连接指示灯 */}
        <div className="flex items-center justify-between pt-2">
          <span className="text-sm text-gray-600">实时连接</span>
          <div className="flex items-center">
            <div className={`w-2 h-2 rounded-full ${
              isConnected ? 'bg-green-500' : 'bg-red-500'
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