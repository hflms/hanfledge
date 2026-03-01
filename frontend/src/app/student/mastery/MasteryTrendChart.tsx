'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import echarts from '@/lib/echarts-setup';

interface MasteryTrendChartProps {
    dates: string[];
    values: number[];
}

export default function MasteryTrendChart({ dates, values }: MasteryTrendChartProps) {
    const option = {
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#1a1a3e',
            borderColor: 'rgba(255,255,255,0.1)',
            textStyle: { color: '#f0f0f5', fontSize: 12 },
            formatter: (params: Array<{ name: string; value: number }>) => {
                const p = params[0];
                return `${p.name}<br/>平均掌握度: ${Math.round(p.value * 100)}%`;
            },
        },
        grid: {
            left: 16,
            right: 16,
            bottom: 24,
            top: 16,
            containLabel: true,
        },
        xAxis: {
            type: 'category' as const,
            data: dates,
            axisLabel: {
                color: '#9898b0',
                fontSize: 11,
            },
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
            axisTick: { show: false },
        },
        yAxis: {
            type: 'value' as const,
            min: 0,
            max: 1,
            axisLabel: {
                color: '#5e5e7a',
                fontSize: 11,
                formatter: (v: number) => `${Math.round(v * 100)}%`,
            },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } },
        },
        series: [{
            type: 'line',
            data: values,
            smooth: true,
            lineStyle: {
                color: '#6c5ce7',
                width: 3,
            },
            areaStyle: {
                color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
                    { offset: 0, color: 'rgba(108, 92, 231, 0.3)' },
                    { offset: 1, color: 'rgba(108, 92, 231, 0.02)' },
                ]),
            },
            itemStyle: {
                color: '#6c5ce7',
            },
            symbolSize: 6,
        }],
    };

    return (
        <ReactEChartsCore
            echarts={echarts}
            option={option}
            style={{ height: 280 }}
            opts={{ renderer: 'canvas' }}
        />
    );
}
