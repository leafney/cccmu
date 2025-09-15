import { useState, useRef, useEffect } from 'react';
import { AlertCircle, Loader2, Check, HelpCircle } from 'lucide-react';
import { useAuth } from '../hooks/useAuth';
import toast from 'react-hot-toast';

export function LoginPage() {
  const [key, setKey] = useState('');
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [error, setError] = useState('');
  const [showTooltip, setShowTooltip] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const { login } = useAuth();

  // 组件挂载时自动聚焦输入框
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.focus();
    }
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    
    if (!key.trim()) {
      setError('请输入访问密钥');
      return;
    }

    setIsLoggingIn(true);
    setError('');

    try {
      const success = await login(key.trim());
      if (!success) {
        setError('访问密钥错误');
        toast.error('访问密钥错误');
      }
    } catch (error) {
      console.error('登录失败:', error);
      setError('登录失败，请重试');
      toast.error('登录失败，请重试');
    } finally {
      setIsLoggingIn(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSubmit(e as any);
    }
  };

  return (
    <div className="min-h-screen bg-gradient-to-br from-gray-900 via-blue-900 to-purple-900 flex items-center justify-center py-12 px-4 sm:px-6 lg:px-8">
      <div className="max-w-md w-full space-y-8">
        {/* Logo和标题 */}
        <div className="text-center">
          <div className="mx-auto h-20 w-20 flex items-center justify-center bg-white/10 backdrop-blur-sm rounded-full border border-white/20 shadow-lg">
            <svg xmlns="http://www.w3.org/2000/svg" width="40" height="40" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <polyline points="22,12 18,12 15,21 9,3 6,12 2,12"/>
            </svg>
          </div>
          <h1 className="mt-6 text-3xl font-bold text-white">
            ACM Claude 积分监控
          </h1>
        </div>

        {/* 登录表单 */}
        <div className="bg-white/10 backdrop-blur-sm rounded-xl border border-white/20 shadow-xl p-8 w-full max-w-lg">
          <form onSubmit={handleSubmit}>
            <div className="relative">
              {/* 帮助图标和浮层 */}
              <div 
                className="absolute left-3 top-1/2 transform -translate-y-1/2 z-10"
                onMouseEnter={() => setShowTooltip(true)}
                onMouseLeave={() => setShowTooltip(false)}
              >
                <HelpCircle className="h-5 w-5 text-white/60 hover:text-white/80 cursor-help transition-colors" />
                
                {/* 提示浮层 */}
                {showTooltip && (
                  <div className="absolute top-full left-0 mt-2 w-64 bg-gray-900/95 backdrop-blur-sm border border-white/20 rounded-lg p-3 text-xs text-white/90 shadow-xl z-20">
                    <div className="font-medium mb-1">如何获取访问密钥？</div>
                    <div className="text-white/70">
                      请在启动 CCCMU 程序的控制台中查看访问密钥。
                      如果忘记密钥，可以删除 .auth 文件重启程序重新生成。
                    </div>
                    {/* 箭头 */}
                    <div className="absolute -top-1 left-4 w-2 h-2 bg-gray-900/95 border-l border-t border-white/20 transform rotate-45"></div>
                  </div>
                )}
              </div>

              {/* 输入框和确认按钮 */}
              <div className="flex shadow-lg">
                <input
                  ref={inputRef}
                  type="password"
                  value={key}
                  onChange={(e) => setKey(e.target.value)}
                  onKeyPress={handleKeyPress}
                  className="flex-1 pl-12 pr-4 py-3 bg-white/15 border-2 border-white/30 placeholder-white/60 text-white rounded-l-xl focus:outline-none focus:ring-4 focus:ring-blue-400/30 focus:border-blue-300/50 focus:bg-white/20 disabled:opacity-70 transition-all duration-300 text-base backdrop-blur-md"
                  placeholder="请输入访问密钥"
                  disabled={isLoggingIn}
                />
                <button
                  type="submit"
                  disabled={isLoggingIn || !key.trim()}
                  className="px-6 bg-gradient-to-r from-blue-500/80 to-purple-500/80 border-2 border-l-0 border-blue-400/50 text-white hover:from-blue-400/90 hover:to-purple-400/90 hover:border-blue-300/60 hover:shadow-lg focus:outline-none focus:ring-4 focus:ring-blue-400/30 disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:shadow-none transition-all duration-300 backdrop-blur-md rounded-r-xl flex items-center justify-center"
                >
                  {isLoggingIn ? (
                    <Loader2 className="h-5 w-5 animate-spin" />
                  ) : (
                    <Check className="h-5 w-5" />
                  )}
                </button>
              </div>
            </div>

            {/* 错误提示 */}
            {error && (
              <div className="flex items-center space-x-2 text-red-300 text-sm bg-red-500/20 border border-red-400/30 rounded-lg p-3 mt-4">
                <AlertCircle className="h-4 w-4 flex-shrink-0" />
                <span>{error}</span>
              </div>
            )}
          </form>
        </div>
      </div>
    </div>
  );
}