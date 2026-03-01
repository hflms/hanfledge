'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import echarts from '@/lib/echarts-setup';

interface MasteryBarChartProps {
    labels: string[];
    values: number[];
}

export default function MasteryBarChart({ labels, values }: MasteryBarChartProps) {
    // Truncate labels longer than 8 chars
    const shortLabels = labels.map(l => l.length > 8 ? l.slice(0, 8) + '...' : l);

    // Color bars by mastery level
    const barColors = values.map(v => {
        if (v >= 0.8) return '#00b894';  // success
        if (v >= 0.5) return '#fdcb6e';  // warning
        return '#e17055';                 // danger
    });

    const option = {
        tooltip: {
            trigger: 'axis',
            backgroundColor: '#1a1a3e',
            borderColor: 'rgba(255,255,255,0.1)',
            textStyle: { color: '#f0f0f5', fontSize: 12 },
            formatter: (params: Array<{ name: string; value: number; dataIndex: number }>) => {
                const p = params[0];
                const fullLabel = labels[p.dataIndex] || p.name;
                return `${fullLabel}: ${Math.round(p.value * 100)}%`;
            },
        },
        grid: {
            left: 16,
            right: 16,
            bottom: 40,
            top: 16,
            containLabel: true,
        },
        xAxis: {
            type: 'category' as const,
            data: shortLabels,
            axisLabel: {
                color: '#9898b0',
                fontSize: 11,
                rotate: labels.length > 6 ? 35 : 0,
            },
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
            axisTick: { show: false },
        },
        yAxis: {
            type: 'value' as const,
            max: 1,
            axisLabel: {
                color: '#5e5e7a',
                fontSize: 11,
                formatter: (v: number) => `${Math.round(v * 100)}%`,
            },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } },
        },
        series: [{
            type: 'bar',
            data: values.map((v, i) => ({
                value: v,
                itemStyle: { color: barColors[i], borderRadius: [4, 4, 0, 0] },
            })),
            barMaxWidth: 40,
        }],
    };

    return (
        <ReactEChartsCore
            echarts={echarts}
            option={option}
            style={{ height: 320 }}
            opts={{ renderer: 'canvas' }}
        />
    );
}
