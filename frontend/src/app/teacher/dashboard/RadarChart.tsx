'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import echarts from '@/lib/echarts-setup';

interface RadarChartProps {
    labels: string[];
    values: number[];
}

export default function RadarChart({ labels, values }: RadarChartProps) {
    // Truncate labels longer than 6 chars
    const shortLabels = labels.map(l => l.length > 6 ? l.slice(0, 6) + '...' : l);

    const option = {
        tooltip: {
            trigger: 'item',
            backgroundColor: '#1a1a3e',
            borderColor: 'rgba(255,255,255,0.1)',
            textStyle: { color: '#f0f0f5', fontSize: 12 },
            formatter: (params: { value: number[] }) => {
                return labels.map((l, i) =>
                    `${l}: ${Math.round((params.value[i] || 0) * 100)}%`
                ).join('<br/>');
            },
        },
        radar: {
            indicator: shortLabels.map(name => ({
                name,
                max: 1,
            })),
            shape: 'polygon',
            splitNumber: 4,
            axisName: {
                color: '#9898b0',
                fontSize: 11,
            },
            splitLine: {
                lineStyle: { color: 'rgba(255,255,255,0.08)' },
            },
            splitArea: {
                show: true,
                areaStyle: {
                    color: ['rgba(108,92,231,0.02)', 'rgba(108,92,231,0.04)'],
                },
            },
            axisLine: {
                lineStyle: { color: 'rgba(255,255,255,0.08)' },
            },
        },
        series: [{
            type: 'radar',
            data: [{
                value: values,
                name: '班级平均掌握度',
                lineStyle: {
                    color: '#6c5ce7',
                    width: 2,
                },
                areaStyle: {
                    color: 'rgba(108, 92, 231, 0.2)',
                },
                itemStyle: {
                    color: '#6c5ce7',
                },
            }],
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
