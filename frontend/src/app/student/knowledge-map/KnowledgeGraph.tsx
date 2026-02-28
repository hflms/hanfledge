'use client';

import { useCallback } from 'react';
import ReactEChartsCore from 'echarts-for-react/lib/core';
import * as echarts from 'echarts/core';
import { GraphChart } from 'echarts/charts';
import { TooltipComponent, LegendComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import type { KnowledgeMapData, KnowledgeMapNode } from '@/lib/api';

echarts.use([GraphChart, TooltipComponent, LegendComponent, CanvasRenderer]);

// -- Color helpers ------------------------------------------------

function masteryColor(mastery: number): string {
    if (mastery < 0) return '#5e5e7a';   // gray — no data
    if (mastery >= 0.8) return '#00b894'; // green
    if (mastery >= 0.5) return '#fdcb6e'; // yellow
    return '#e17055';                     // red
}

function masteryLabel(mastery: number): string {
    if (mastery < 0) return '暂无数据';
    return `${Math.round(mastery * 100)}%`;
}

function nodeSize(node: KnowledgeMapNode): number {
    const base = node.is_key_point ? 28 : 18;
    return base + (node.difficulty || 1) * 2;
}

// -- Component ----------------------------------------------------

interface KnowledgeGraphProps {
    data: KnowledgeMapData;
}

export default function KnowledgeGraph({ data }: KnowledgeGraphProps) {
    const buildOption = useCallback((mapData: KnowledgeMapData) => {
        const idxMap = new Map<string, number>();
        mapData.nodes.forEach((n, i) => idxMap.set(n.neo4j_id, i));

        const graphNodes = mapData.nodes.map((n) => ({
            name: n.title,
            id: n.neo4j_id,
            symbolSize: nodeSize(n),
            itemStyle: {
                color: masteryColor(n.mastery),
                borderColor: n.is_key_point ? '#a29bfe' : 'rgba(255,255,255,0.15)',
                borderWidth: n.is_key_point ? 3 : 1,
            },
            label: {
                show: true,
                fontSize: 11,
                color: '#f0f0f5',
                formatter: n.title.length > 6 ? n.title.slice(0, 6) + '...' : n.title,
            },
            value: n.mastery,
            category: n.chapter_title,
            _raw: n,
        }));

        const graphEdges = mapData.edges
            .filter((e) => idxMap.has(e.source) && idxMap.has(e.target))
            .map((e) => ({
                source: e.source,
                target: e.target,
                lineStyle: {
                    color: e.type === 'REQUIRES' ? 'rgba(108, 92, 231, 0.5)' : 'rgba(152, 152, 176, 0.3)',
                    width: e.type === 'REQUIRES' ? 2 : 1,
                    type: e.type === 'REQUIRES' ? ('solid' as const) : ('dashed' as const),
                },
                symbol: e.type === 'REQUIRES' ? ['none', 'arrow'] : ['none', 'none'],
                symbolSize: 8,
            }));

        const chapters = [...new Set(mapData.nodes.map((n) => n.chapter_title))];

        return {
            tooltip: {
                trigger: 'item' as const,
                backgroundColor: '#1a1a3e',
                borderColor: 'rgba(255,255,255,0.1)',
                textStyle: { color: '#f0f0f5', fontSize: 13 },
                formatter: (params: { dataType: string; data: { _raw?: KnowledgeMapNode; name?: string } }) => {
                    if (params.dataType === 'node' && params.data._raw) {
                        const n = params.data._raw;
                        return [
                            `<b>${n.title}</b>`,
                            `章节: ${n.chapter_title}`,
                            `难度: ${'★'.repeat(n.difficulty)}${'☆'.repeat(5 - n.difficulty)}`,
                            `掌握度: ${masteryLabel(n.mastery)}`,
                            `练习次数: ${n.attempt_count}`,
                            n.is_key_point ? '<span style="color:#a29bfe">★ 重点知识</span>' : '',
                        ]
                            .filter(Boolean)
                            .join('<br/>');
                    }
                    return params.data.name || '';
                },
            },
            series: [
                {
                    type: 'graph',
                    layout: 'force',
                    roam: true,
                    draggable: true,
                    data: graphNodes,
                    edges: graphEdges,
                    categories: chapters.map((c) => ({ name: c })),
                    force: {
                        repulsion: 200,
                        edgeLength: [80, 160],
                        gravity: 0.1,
                        friction: 0.6,
                        layoutAnimation: true,
                    },
                    emphasis: {
                        focus: 'adjacency',
                        lineStyle: { width: 3 },
                    },
                    lineStyle: {
                        curveness: 0.1,
                    },
                },
            ],
            animationDuration: 800,
            animationEasingUpdate: 'quinticInOut',
        };
    }, []);

    return (
        <ReactEChartsCore
            echarts={echarts}
            option={buildOption(data)}
            style={{ height: 520 }}
            opts={{ renderer: 'canvas' }}
        />
    );
}
