import { useMemo } from 'react';
import ReactECharts from 'echarts-for-react';
import type { IUsageData } from '../types';

interface UsageChartProps {
  data: IUsageData[];
  className?: string;
}

export function UsageChart({ data, className = '' }: UsageChartProps) {
  const chartData = useMemo(() => {
    if (!data || data.length === 0) {
      return {
        times: [],
        series: {}
      };
    }

    // 按模型分组数据
    const groupedData: { [key: string]: IUsageData[] } = {};
    data.forEach(item => {
      if (item.creditsUsed > 0) {
        if (!groupedData[item.model]) {
          groupedData[item.model] = [];
        }
        groupedData[item.model].push(item);
      }
    });

    // 生成时间轴
    const timeSet = new Set<string>();
    Object.values(groupedData).forEach(items => {
      items.forEach(item => {
        timeSet.add(new Date(item.createdAt).toISOString());
      });
    });
    
    const times = Array.from(timeSet).sort();

    // 为每个模型创建数据系列
    const series: { [key: string]: number[] } = {};
    Object.keys(groupedData).forEach(model => {
      series[model] = times.map(time => {
        const item = groupedData[model].find(d => 
          new Date(d.createdAt).toISOString() === time
        );
        return item ? item.creditsUsed : 0;
      });
    });

    return { times, series };
  }, [data]);

  const option = useMemo(() => ({
    backgroundColor: 'transparent',
    title: {
      text: 'Claude Code积分使用趋势',
      left: 'center',
      textStyle: {
        fontSize: 20,
        fontWeight: 'normal',
        color: '#ffffff'
      }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: 'rgba(0, 0, 0, 0.8)',
      borderColor: 'rgba(255, 255, 255, 0.2)',
      textStyle: {
        color: '#ffffff'
      },
      formatter: (params: any[]) => {
        // 格式化时间显示，显示完整的日期和时间
        const date = new Date(params[0].name);
        const formattedTime = date.toLocaleString('zh-CN', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit',
          hour12: false
        });
        
        let tooltip = `<div style="margin-bottom: 8px; font-weight: bold; color: #fbbf24;">${formattedTime}</div>`;
        params.forEach(param => {
          if (param.value > 0) {
            tooltip += `<div style="margin: 4px 0;">${param.marker}<span style="margin-right: 8px;">${param.seriesName}</span><span style="font-weight: bold; color: #fbbf24;">${param.value} credits</span></div>`;
          }
        });
        return tooltip;
      }
    },
    legend: {
      top: '40px',
      data: Object.keys(chartData.series),
      textStyle: {
        color: '#ffffff'
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '5%',
      top: '100px',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: chartData.times.map(time => new Date(time).toLocaleTimeString()),
      axisLabel: {
        rotate: 45,
        fontSize: 12,
        color: '#ffffff'
      },
      axisLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.3)'
        }
      },
      splitLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.1)'
        }
      }
    },
    yAxis: {
      type: 'value',
      name: 'Credits',
      nameTextStyle: {
        color: '#ffffff'
      },
      axisLabel: {
        formatter: '{value}',
        color: '#ffffff'
      },
      axisLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.3)'
        }
      },
      splitLine: {
        lineStyle: {
          color: 'rgba(255, 255, 255, 0.1)'
        }
      }
    },
    series: Object.keys(chartData.series).map((model, index) => {
      const colors = [
        '#60A5FA', // blue-400
        '#34D399', // emerald-400
        '#F59E0B', // amber-500
        '#EF4444', // red-500
        '#8B5CF6', // violet-500
        '#EC4899', // pink-500
      ];
      return {
        name: model,
        type: 'line',
        data: chartData.series[model],
        smooth: true,
        symbol: 'circle',
        symbolSize: 8,
        showSymbol: true,
        lineStyle: {
          width: 3,
          color: colors[index % colors.length]
        },
        areaStyle: {
          opacity: 0.2,
          color: colors[index % colors.length]
        },
        itemStyle: {
          color: colors[index % colors.length],
          borderWidth: 2,
          borderColor: '#fff'
        },
        label: {
          show: true,
          position: 'top',
          formatter: (params: any) => {
            // 只在积分大于0时显示标签
            return params.value > 0 ? `${params.value}` : '';
          },
          color: colors[index % colors.length],
          fontSize: 11,
          fontWeight: 'bold',
          backgroundColor: 'rgba(0, 0, 0, 0.6)',
          borderRadius: 4,
          padding: [2, 6],
          borderWidth: 1,
          borderColor: colors[index % colors.length]
        },
        emphasis: {
          focus: 'series',
          label: {
            fontSize: 12
          },
          itemStyle: {
            shadowBlur: 10,
            shadowColor: colors[index % colors.length]
          }
        }
      };
    }),
    animation: true,
    animationDuration: 1000
  }), [chartData]);

  if (!data || data.length === 0) {
    return (
      <div className={`@container flex items-center justify-center bg-white/5 backdrop-blur-sm rounded-2xl border border-white/10 ${className}`}>
        <div className="text-center space-y-4 p-8">
          <div className="w-20 h-20 mx-auto rounded-full bg-white/10 flex items-center justify-center mb-6">
            <svg className="w-10 h-10 text-white/60" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
            </svg>
          </div>
          <div className="text-white text-xl font-medium">暂无数据</div>
          <div className="text-white/70 text-base max-w-sm mx-auto leading-relaxed">
            请先配置Cookie并启动监控任务
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className={`@container bg-white/5 backdrop-blur-sm rounded-2xl border border-white/10 overflow-hidden ${className}`}>
      <ReactECharts 
        option={option} 
        style={{ 
          height: '100%',
          width: '100%',
          minHeight: '400px'
        }}
        opts={{ renderer: 'canvas' }}
      />
    </div>
  );
}