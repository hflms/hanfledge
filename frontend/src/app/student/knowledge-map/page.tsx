'use client';

import { useEffect, useState, useCallback } from 'react';
import ReactEChartsCore from 'echarts-for-react/lib/core';
import * as echarts from 'echarts/core';
import { GraphChart } from 'echarts/charts';
import { TooltipComponent, LegendComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import {
    listCourses,
    getStudentKnowledgeMap,
    type Course,
    type KnowledgeMapData,
    type KnowledgeMapNode,
} from '@/lib/api';
import DashboardLayout from '@/components/DashboardLayout';
import styles from './page.module.css';

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
    // Scale slightly by difficulty (1-5)
    return base + (node.difficulty || 1) * 2;
}

// -- Component ----------------------------------------------------

export default function KnowledgeMapPage() {
    const [courses, setCourses] = useState<Course[]>([]);
    const [selectedCourseId, setSelectedCourseId] = useState<number | null>(null);
    const [mapData, setMapData] = useState<KnowledgeMapData | null>(null);
    const [loading, setLoading] = useState(true);
    const [mapLoading, setMapLoading] = useState(false);

    // Load courses on mount
    useEffect(() => {
        listCourses()
            .then((list) => {
                setCourses(list);
                if (list.length > 0) {
                    setSelectedCourseId(list[0].id);
                }
            })
            .catch(console.error)
            .finally(() => setLoading(false));
    }, []);

    // Load knowledge map when course changes
    const fetchMap = useCallback(async (courseId: number) => {
        setMapLoading(true);
        try {
            const data = await getStudentKnowledgeMap(courseId);
            setMapData(data);
        } catch (err) {
            console.error('Failed to load knowledge map:', err);
            setMapData(null);
        } finally {
            setMapLoading(false);
        }
    }, []);

    useEffect(() => {
        if (selectedCourseId) {
            fetchMap(selectedCourseId);
        }
    }, [selectedCourseId, fetchMap]);

    // -- ECharts option builder -----------------------------------

    const buildOption = useCallback((data: KnowledgeMapData) => {
        // Build a neo4j_id → index lookup
        const idxMap = new Map<string, number>();
        data.nodes.forEach((n, i) => idxMap.set(n.neo4j_id, i));

        const graphNodes = data.nodes.map((n) => ({
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
            // Store extra data for tooltip
            value: n.mastery,
            category: n.chapter_title,
            _raw: n,
        }));

        const graphEdges = data.edges
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

        // Collect unique chapters for categories
        const chapters = [...new Set(data.nodes.map((n) => n.chapter_title))];

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

    // -- Render ----------------------------------------------------

    if (loading) {
        return (
            <DashboardLayout variant="student">
                <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                    <div className="spinner" />
                </div>
            </DashboardLayout>
        );
    }

    if (courses.length === 0) {
        return (
            <DashboardLayout variant="student">
                <div className="fade-in">
                    <div className={styles.pageHeader}>
                        <h1 className={styles.pageTitle}>知识图谱</h1>
                    </div>
                    <div className={styles.emptyState}>
                        <div className={styles.emptyIcon}>🗺️</div>
                        <div className={styles.emptyText}>暂无课程数据</div>
                    </div>
                </div>
            </DashboardLayout>
        );
    }

    return (
        <DashboardLayout variant="student">
            <div className="fade-in">
                {/* Header with course selector */}
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>知识图谱</h1>
                    {courses.length > 1 && (
                        <select
                            className={styles.courseSelect}
                            value={selectedCourseId || ''}
                            onChange={(e) => setSelectedCourseId(Number(e.target.value))}
                        >
                            {courses.map((c) => (
                                <option key={c.id} value={c.id}>
                                    {c.title}
                                </option>
                            ))}
                        </select>
                    )}
                </div>

                {mapLoading && (
                    <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                        <div className="spinner" />
                    </div>
                )}

                {!mapLoading && mapData && mapData.nodes.length > 0 && (
                    <>
                        {/* Summary Cards */}
                        <div className={styles.summaryRow}>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>知识点总数</div>
                                <div className={styles.summaryValue}>
                                    {mapData.nodes.length}
                                    <span className={styles.summaryUnit}>个</span>
                                </div>
                            </div>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>平均掌握度</div>
                                <div className={styles.summaryValue}>
                                    {mapData.avg_mastery >= 0
                                        ? Math.round(mapData.avg_mastery * 100)
                                        : '--'}
                                    <span className={styles.summaryUnit}>
                                        {mapData.avg_mastery >= 0 ? '%' : ''}
                                    </span>
                                </div>
                            </div>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>已掌握</div>
                                <div className={styles.summaryValue}>
                                    {mapData.mastered_count}
                                    <span className={styles.summaryUnit}>
                                        / {mapData.nodes.length}
                                    </span>
                                </div>
                            </div>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>待加强</div>
                                <div className={styles.summaryValue}>
                                    {mapData.weak_count}
                                    <span className={styles.summaryUnit}>个</span>
                                </div>
                            </div>
                        </div>

                        {/* Graph */}
                        <div className={styles.graphCard}>
                            <div className={styles.graphHeader}>
                                <div className={styles.graphTitle}>
                                    {mapData.course_title} — 知识点关系图
                                </div>
                                <div className={styles.legend}>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#00b894' }}
                                        />
                                        已掌握
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#fdcb6e' }}
                                        />
                                        学习中
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#e17055' }}
                                        />
                                        待加强
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#5e5e7a' }}
                                        />
                                        暂无数据
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendLine}
                                            style={{ background: 'rgba(108, 92, 231, 0.5)' }}
                                        />
                                        前置依赖
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendLineDashed}
                                            style={{ color: 'rgba(152, 152, 176, 0.5)' }}
                                        />
                                        关联关系
                                    </div>
                                </div>
                            </div>
                            <ReactEChartsCore
                                echarts={echarts}
                                option={buildOption(mapData)}
                                style={{ height: 520 }}
                                opts={{ renderer: 'canvas' }}
                            />
                        </div>
                    </>
                )}

                {!mapLoading && mapData && mapData.nodes.length === 0 && (
                    <div className={styles.emptyState}>
                        <div className={styles.emptyIcon}>🗺️</div>
                        <div className={styles.emptyText}>
                            该课程暂无知识点数据，请等待教师完成课程内容配置
                        </div>
                    </div>
                )}

                {!mapLoading && !mapData && (
                    <div className={styles.emptyState}>
                        <div className={styles.emptyIcon}>⚠️</div>
                        <div className={styles.emptyText}>加载知识图谱失败，请稍后重试</div>
                    </div>
                )}
            </div>
        </DashboardLayout>
    );
}
