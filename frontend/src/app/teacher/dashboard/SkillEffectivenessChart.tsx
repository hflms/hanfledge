'use client';

import ReactEChartsCore from 'echarts-for-react/lib/core';
import echarts from '@/lib/echarts-setup';
import type { SkillEffectivenessItem } from '@/lib/api';

// -- Skill Label Helpers --------------------------------------

const SKILL_LABELS: Record<string, string> = {
    general_concept_socratic: '苏格拉底式',
    general_assessment_fallacy: '谬误侦探',
};

function shortSkillLabel(skillId: string): string {
    if (SKILL_LABELS[skillId]) return SKILL_LABELS[skillId];
    // Take last segment
    const parts = skillId.split('_');
    return parts[parts.length - 1];
}

// -- Chart Component ------------------------------------------

interface SkillEffectivenessChartProps {
    items: SkillEffectivenessItem[];
}

export default function SkillEffectivenessChart({ items }: SkillEffectivenessChartProps) {
    if (items.length === 0) {
        return (
            <div style={{ textAlign: 'center', padding: 40, color: 'var(--text-muted)', fontSize: 14 }}>
                暂无技能评估数据
            </div>
        );
    }

    const labels = items.map(item => shortSkillLabel(item.skill_id));
    const dimensions = [
        { key: 'avg_faithfulness' as const, label: '忠实度', color: '#6c5ce7' },
        { key: 'avg_actionability' as const, label: '可执行性', color: '#00b894' },
        { key: 'avg_answer_restraint' as const, label: '答案克制', color: '#e17055' },
        { key: 'avg_context_precision' as const, label: '上下文精度', color: '#fdcb6e' },
        { key: 'avg_context_recall' as const, label: '上下文召回', color: '#74b9ff' },
    ];

    const series = dimensions.map(dim => ({
        name: dim.label,
        type: 'bar' as const,
        data: items.map(item => Number((item[dim.key] * 100).toFixed(1))),
        itemStyle: { color: dim.color },
        barMaxWidth: 20,
    }));

    const option = {
        tooltip: {
            trigger: 'axis' as const,
            axisPointer: { type: 'shadow' as const },
            backgroundColor: '#1a1a3e',
            borderColor: 'rgba(255,255,255,0.1)',
            textStyle: { color: '#f0f0f5', fontSize: 12 },
            formatter: (params: Array<{ seriesName: string; value: number; marker: string }>) => {
                const skillIdx = 0;
                const skillId = items[skillIdx]?.skill_id || '';
                let result = `<strong>${skillId}</strong><br/>`;
                params.forEach(p => {
                    result += `${p.marker} ${p.seriesName}: ${p.value}%<br/>`;
                });
                return result;
            },
        },
        legend: {
            top: 0,
            textStyle: { color: '#9898b0', fontSize: 11 },
            itemWidth: 12,
            itemHeight: 8,
        },
        grid: {
            top: 40,
            left: 50,
            right: 20,
            bottom: 40,
        },
        xAxis: {
            type: 'category' as const,
            data: labels,
            axisLabel: { color: '#9898b0', fontSize: 11 },
            axisLine: { lineStyle: { color: 'rgba(255,255,255,0.08)' } },
        },
        yAxis: {
            type: 'value' as const,
            max: 100,
            axisLabel: {
                color: '#9898b0',
                fontSize: 11,
                formatter: '{value}%',
            },
            splitLine: { lineStyle: { color: 'rgba(255,255,255,0.06)' } },
        },
        series,
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
