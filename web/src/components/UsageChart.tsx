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
    title: {
      text: 'Claude Code积分使用趋势',
      left: 'center',
      textStyle: {
        fontSize: 16,
        fontWeight: 'normal'
      }
    },
    tooltip: {
      trigger: 'axis',
      formatter: (params: any[]) => {
        let tooltip = `<div>${new Date(params[0].name).toLocaleString()}</div>`;
        params.forEach(param => {
          if (param.value > 0) {
            tooltip += `<div>${param.marker}${param.seriesName}: ${param.value} credits</div>`;
          }
        });
        return tooltip;
      }
    },
    legend: {
      top: '30px',
      data: Object.keys(chartData.series)
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '3%',
      top: '80px',
      containLabel: true
    },
    xAxis: {
      type: 'category',
      data: chartData.times.map(time => new Date(time).toLocaleTimeString()),
      axisLabel: {
        rotate: 45,
        fontSize: 10
      }
    },
    yAxis: {
      type: 'value',
      name: 'Credits',
      axisLabel: {
        formatter: '{value}'
      }
    },
    series: Object.keys(chartData.series).map(model => ({
      name: model,
      type: 'line',
      data: chartData.series[model],
      smooth: true,
      symbol: 'circle',
      symbolSize: 4,
      lineStyle: {
        width: 2
      },
      areaStyle: {
        opacity: 0.1
      }
    })),
    animation: true,
    animationDuration: 1000
  }), [chartData]);

  if (!data || data.length === 0) {
    return (
      <div className={`flex items-center justify-center h-96 bg-gray-50 rounded-lg border-2 border-dashed border-gray-300 ${className}`}>
        <div className="text-center">
          <div className="text-gray-500 text-lg mb-2">暂无数据</div>
          <div className="text-gray-400 text-sm">请先配置Cookie并启动监控任务</div>
        </div>
      </div>
    );
  }

  return (
    <div className={`bg-white rounded-lg shadow-sm border ${className}`}>
      <ReactECharts 
        option={option} 
        style={{ height: '400px', width: '100%' }}
        opts={{ renderer: 'canvas' }}
      />
    </div>
  );
}