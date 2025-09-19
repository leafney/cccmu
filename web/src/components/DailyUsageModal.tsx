import { useState, useEffect, useRef } from 'react';
import { X, BarChart3 } from 'lucide-react';
import * as echarts from 'echarts/core';
import { BarChart } from 'echarts/charts';
import {
  TooltipComponent,
  GridComponent,
  LegendComponent
} from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import type { IDailyUsage } from '../types';

// 注册必需的组件
echarts.use([
  BarChart,
  TooltipComponent,
  GridComponent,
  LegendComponent,
  CanvasRenderer
]);

interface DailyUsageModalProps {
  isOpen: boolean;
  onClose: () => void;
  data: IDailyUsage[];
}

export function DailyUsageModal({ isOpen, onClose, data }: DailyUsageModalProps) {
  const [weeklyUsage, setWeeklyUsage] = useState<IDailyUsage[]>([]);
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);

  // 当外部数据变化时更新内部状态
  useEffect(() => {
    if (data && data.length > 0) {
      setWeeklyUsage(data);
    }
  }, [data]);

  // 初始化图表
  const initChart = () => {
    if (!chartRef.current || weeklyUsage.length === 0) return;

    // 销毁现有图表实例
    if (chartInstance.current) {
      chartInstance.current.dispose();
    }

    // 创建新的图表实例
    chartInstance.current = echarts.init(chartRef.current);

    // 准备图表数据
    const dates = weeklyUsage.map(item => {
      const date = new Date(item.date);
      // 格式化为月/日显示
      return `${date.getMonth() + 1}/${date.getDate()}`;
    });

    // 获取所有使用过的模型列表
    const allModels = new Set<string>();
    weeklyUsage.forEach(item => {
      if (item.modelCredits) {
        Object.keys(item.modelCredits).forEach(model => {
          if (item.modelCredits[model] > 0) {
            allModels.add(model);
          }
        });
      }
    });

    const modelList = Array.from(allModels);

    // 模型颜色映射
    const modelColors: { [key: string]: string } = {
      'claude-sonnet-4-20250514': '#3B82F6',    // 蓝色
      'gpt-5-codex': '#F97316',       // 橙色  
    };

    // 为未知模型分配默认颜色
    const defaultColors = [
      '#10B981', '#8B5CF6', '#F59E0B', '#EF4444', '#06B6D4',
      '#64748B', '#84CC16', '#EC4899', '#14B8A6', '#F472B6',
      '#8B5A2B', '#6366F1', '#DC2626', '#059669', '#7C3AED',
      '#EA580C', '#0891B2', '#BE185D', '#7C2D12', '#374151',
      '#4338CA', '#16A34A', '#CA8A04', '#C2410C', '#BE123C',
      '#15803D', '#1E40AF', '#9333EA', '#0369A1', '#DC2626'
    ];
    let colorIndex = 0;

    // 计算总积分最大值用于Y轴范围设置
    const maxValue = Math.max(...weeklyUsage.map(item => item.totalCredits));
    const yAxisMax = maxValue > 0 ? Math.ceil(maxValue * 1.2) : 10;

    // 图表配置
    const option = {
      title: {
        text: '最近一周每日积分使用量',
        left: 'center',
        top: '2%',
        textStyle: {
          color: '#374151',
          fontSize: 14,
          fontWeight: 600
        }
      },
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow'
        },
        backgroundColor: 'rgba(50, 50, 93, 0.9)',
        borderColor: 'rgba(50, 50, 93, 0.9)',
        textStyle: {
          color: '#fff'
        },
        formatter: (params: any) => {
          if (!params || params.length === 0) return '';
          
          const dataIndex = params[0].dataIndex;
          const originalDate = weeklyUsage[dataIndex]?.date;
          const formattedDate = originalDate ? new Date(originalDate).toLocaleDateString('zh-CN') : '';
          
          let tooltip = `${formattedDate}<br/>`;
          let totalCredits = 0;
          
          // 显示每个模型的积分
          params.forEach((param: any) => {
            if (param.value > 0) {
              tooltip += `${param.seriesName}: ${param.value.toLocaleString()}<br/>`;
              totalCredits += param.value;
            }
          });
          
          tooltip += `<strong>总计: ${totalCredits.toLocaleString()}</strong>`;
          return tooltip;
        }
      },
      legend: {
        type: 'scroll',
        orient: 'horizontal',
        top: '8%',
        left: 'center',
        itemWidth: 14,
        itemHeight: 14,
        textStyle: {
          color: '#6B7280',
          fontSize: 12
        }
      },
      grid: {
        left: '8%',
        right: '8%',
        top: '18%',  // 增加顶部空间以容纳图例
        bottom: '12%',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: dates,
        axisLabel: {
          color: '#6B7280',
          fontSize: 12
        },
        axisLine: {
          lineStyle: {
            color: '#E5E7EB'
          }
        },
        axisTick: {
          alignWithLabel: true
        }
      },
      yAxis: {
        type: 'value',
        name: '积分',
        max: yAxisMax,
        nameTextStyle: {
          color: '#6B7280',
          fontSize: 12
        },
        axisLabel: {
          color: '#6B7280',
          fontSize: 11,
          formatter: (value: number) => {
            if (value >= 1000) {
              return (value / 1000).toFixed(1) + 'K';
            }
            return value.toString();
          }
        },
        axisLine: {
          lineStyle: {
            color: '#E5E7EB'
          }
        },
        splitLine: {
          lineStyle: {
            color: '#F3F4F6',
            type: 'dashed'
          }
        }
      },
      series: modelList.map((model, index) => {
        // 为每个模型准备数据
        const modelData = weeklyUsage.map(item => {
          return item.modelCredits?.[model] || 0;
        });

        // 获取模型颜色
        const modelColor = modelColors[model] || defaultColors[colorIndex++ % defaultColors.length];

        return {
          name: model,
          type: 'bar',
          stack: 'total',  // 关键：启用堆叠
          data: modelData,
          itemStyle: {
            color: modelColor,
            borderRadius: index === modelList.length - 1 ? [4, 4, 0, 0] : [0, 0, 0, 0]  // 只有最顶部的柱子有圆角
          },
          emphasis: {
            itemStyle: {
              color: modelColor,
              opacity: 0.8
            }
          },
          barWidth: '50%',
          label: {
            show: false  // 在堆叠图中隐藏单个标签，避免重叠
          } as any
        };
      }).concat([
        // 添加一个透明的系列用于显示总计标签
        {
          name: '',
          type: 'bar',
          stack: 'total',
          data: weeklyUsage.map(() => 0),  // 数据为0，不显示柱子
          itemStyle: {
            color: 'transparent',
            borderRadius: [0, 0, 0, 0]
          },
          emphasis: {
            itemStyle: {
              color: 'transparent',
              opacity: 1
            }
          },
          barWidth: '50%',
          label: {
            show: true,
            position: 'top',
            color: '#374151',
            fontSize: 10,
            formatter: (params: any) => {
              const total = weeklyUsage[params.dataIndex]?.totalCredits || 0;
              return total > 0 ? total.toLocaleString() : '';
            }
          },
          silent: true,  // 不响应鼠标事件
          tooltip: {
            show: false
          }
        } as any
      ])
    };

    chartInstance.current.setOption(option, true);
    
    // 确保图表能够响应容器大小变化
    setTimeout(() => {
      if (chartInstance.current) {
        chartInstance.current.resize();
      }
    }, 100);
  };


  // 当数据加载完成后初始化图表
  useEffect(() => {
    if (isOpen && weeklyUsage.length > 0) {
      const timer = setTimeout(() => {
        initChart();
      }, 200); // 增加延迟确保DOM已更新

      return () => clearTimeout(timer);
    }
  }, [isOpen, weeklyUsage]);

  // 监听窗口大小变化和弹窗打开状态
  useEffect(() => {
    const handleWindowResize = () => {
      if (isOpen && chartInstance.current) {
        setTimeout(() => {
          chartInstance.current?.resize();
        }, 100);
      }
    };

    window.addEventListener('resize', handleWindowResize);
    
    // 弹窗打开时也触发一次resize
    if (isOpen && chartInstance.current) {
      setTimeout(() => {
        chartInstance.current?.resize();
      }, 300);
    }

    return () => window.removeEventListener('resize', handleWindowResize);
  }, [isOpen]);

  // 清理图表实例
  useEffect(() => {
    return () => {
      if (chartInstance.current) {
        chartInstance.current.dispose();
      }
    };
  }, []);

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-y-auto">
      {/* 背景遮罩 */}
      <div 
        className="fixed inset-0 bg-black bg-opacity-50 transition-opacity"
        onClick={onClose}
      />
      
      {/* 弹窗内容 */}
      <div className="flex min-h-full items-center justify-center p-2 md:p-4">
        <div className="relative bg-white rounded-2xl shadow-xl w-full max-w-4xl max-h-[95vh] flex flex-col transform transition-all">
          {/* 标题栏 */}
          <div className="flex items-center justify-between p-4 pb-2 border-b border-gray-200">
            <div className="flex items-center">
              <BarChart3 className="w-5 h-5 mr-2 text-blue-600" />
              <h2 className="text-lg font-semibold text-gray-900">每日积分使用统计</h2>
            </div>
            <button
              onClick={onClose}
              className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
            >
              <X className="w-5 h-5" />
            </button>
          </div>

          {/* 内容区域 */}
          <div className="flex-1 flex flex-col min-h-[480px] overflow-hidden">
            {weeklyUsage.length === 0 && (
              <div className="flex items-center justify-center flex-1 p-6">
                <div className="text-center">
                  <div className="mb-4">
                    <BarChart3 className="w-16 h-16 mx-auto text-gray-300" />
                  </div>
                  <h3 className="text-lg font-medium text-gray-900 mb-2">暂无数据</h3>
                  <p className="text-gray-600">还没有每日积分使用统计数据</p>
                </div>
              </div>
            )}

            {weeklyUsage.length > 0 && (
              <>
                {/* 图表容器 */}
                <div className="flex-1 p-4 pt-2 pb-0">
                  <div 
                    ref={chartRef} 
                    className="w-full h-full"
                    style={{ minHeight: '400px', height: '400px' }}
                  />
                </div>

                {/* 统计信息 */}
                <div className="px-4 pb-2 -mt-2">
                  <div className="grid grid-cols-3 gap-4 pt-2 border-t border-gray-200">
                    <div className="text-center">
                      <div className="text-xl font-bold text-blue-600">
                        {weeklyUsage.reduce((sum, item) => sum + item.totalCredits, 0).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-600 mt-0.5">本周总计</div>
                    </div>
                    <div className="text-center">
                      <div className="text-xl font-bold text-green-600">
                        {Math.round(weeklyUsage.reduce((sum, item) => sum + item.totalCredits, 0) / 7).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-600 mt-0.5">日均使用</div>
                    </div>
                    <div className="text-center">
                      <div className="text-xl font-bold text-purple-600">
                        {Math.max(...weeklyUsage.map(item => item.totalCredits)).toLocaleString()}
                      </div>
                      <div className="text-xs text-gray-600 mt-0.5">单日最高</div>
                    </div>
                  </div>
                </div>
              </>
            )}
          </div>

          {/* 底部提示 */}
          {weeklyUsage.length > 0 && (
            <div className="px-4 py-1.5 border-t border-gray-200 bg-gray-50">
              <p className="text-xs text-gray-500 text-center">
                显示最近7天的每日积分使用量统计，启用历史统计功能后数据每小时更新
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}