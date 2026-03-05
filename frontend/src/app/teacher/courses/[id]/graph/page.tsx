'use client';

import { useParams } from 'next/navigation';
import Link from 'next/link';
import dynamic from 'next/dynamic';
import { type KnowledgeMapData } from '@/lib/api';
import { useApi } from '@/lib/useApi';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

const KnowledgeGraph = dynamic(() => import('@/app/student/knowledge-map/KnowledgeGraph'), { ssr: false });

export default function CourseGraphPage() {
    const params = useParams();
    const courseId = Number(params.id);

    // Fetch the knowledge graph data for the course
    const { data: mapData, isLoading } = useApi<KnowledgeMapData>(`/courses/${courseId}/graph`);

    if (isLoading) {
        return <LoadingSpinner />;
    }

    return (
        <div className="fade-in">
            <Link href={`/teacher/courses/${courseId}/outline`} className={styles.backLink}>
                ← 返回课程大纲
            </Link>

            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>{mapData?.course_title || '课程知识图谱'}</h1>
                <div className={styles.headerActions}>
                    <Link href={`/teacher/courses/${courseId}/materials`} className="btn btn-secondary">
                        📤 教材管理
                    </Link>
                </div>
            </div>

            {!mapData || mapData.nodes.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>🗺️</div>
                    <div className={styles.emptyText}>
                        该课程暂无知识点数据，请先上传教材或配置大纲
                    </div>
                </div>
            ) : (
                <div className={styles.graphCard}>
                    <div className={styles.graphHeader}>
                        <div className={styles.graphTitle}>
                            共包含 {mapData.nodes.length} 个知识点，{mapData.edges.length} 条关系
                        </div>
                        <div className={styles.legend}>
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
                    <div style={{ flex: 1, minHeight: '500px' }}>
                        <KnowledgeGraph data={mapData} />
                    </div>
                </div>
            )}
        </div>
    );
}
