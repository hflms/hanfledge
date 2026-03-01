'use client';

import { useEffect, useState, useCallback } from 'react';
import dynamic from 'next/dynamic';
import {
    listCourses,
    getStudentKnowledgeMap,
    type Course,
    type KnowledgeMapData,
} from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

const KnowledgeGraph = dynamic(() => import('./KnowledgeGraph'), { ssr: false });

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
            .then((data) => {
                const list = data?.items || [];
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

    // -- Render ----------------------------------------------------

    if (loading) {
        return <LoadingSpinner />;
    }

    if (courses.length === 0) {
        return (
            <div className="fade-in">
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>知识图谱</h1>
                </div>
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>🗺️</div>
                    <div className={styles.emptyText}>暂无课程数据</div>
                </div>
            </div>
        );
    }

    return (
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
                    <LoadingSpinner />
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
                                            aria-hidden="true"
                                        />
                                        已掌握
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#fdcb6e' }}
                                            aria-hidden="true"
                                        />
                                        学习中
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#e17055' }}
                                            aria-hidden="true"
                                        />
                                        待加强
                                    </div>
                                    <div className={styles.legendItem}>
                                        <span
                                            className={styles.legendDot}
                                            style={{ background: '#5e5e7a' }}
                                            aria-hidden="true"
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
                            <KnowledgeGraph data={mapData} />
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
    );
}
