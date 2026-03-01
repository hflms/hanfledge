'use client';

import { useEffect, useState, useCallback } from 'react';
import { getErrorNotebook, type ErrorNotebookData, type ErrorNotebookItem } from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

// -- Filter type --------------------------------------------------

type FilterMode = 'all' | 'unresolved' | 'resolved';

// -- Component ----------------------------------------------------

export default function ErrorNotebookPage() {
    const [data, setData] = useState<ErrorNotebookData | null>(null);
    const [loading, setLoading] = useState(true);
    const [filter, setFilter] = useState<FilterMode>('all');
    const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set());

    const fetchData = useCallback(async (mode: FilterMode) => {
        setLoading(true);
        try {
            const opts: { resolved?: boolean } = {};
            if (mode === 'unresolved') opts.resolved = false;
            if (mode === 'resolved') opts.resolved = true;
            const result = await getErrorNotebook(opts);
            setData(result);
        } catch (err) {
            console.error('Failed to load error notebook:', err);
            setData(null);
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchData(filter);
    }, [filter, fetchData]);

    const toggleExpand = (id: number) => {
        setExpandedIds((prev) => {
            const next = new Set(prev);
            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }
            return next;
        });
    };

    const truncateText = (text: string, maxLen: number) => {
        if (text.length <= maxLen) return text;
        return text.slice(0, maxLen) + '...';
    };

    // -- Render ----------------------------------------------------

    return (
        <div className="fade-in">
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>错题本</h1>
                    <div className={styles.filterBar}>
                        <button
                            className={`${styles.filterBtn} ${filter === 'all' ? styles.filterBtnActive : ''}`}
                            onClick={() => setFilter('all')}
                        >
                            全部
                        </button>
                        <button
                            className={`${styles.filterBtn} ${filter === 'unresolved' ? styles.filterBtnActive : ''}`}
                            onClick={() => setFilter('unresolved')}
                        >
                            待解决
                        </button>
                        <button
                            className={`${styles.filterBtn} ${filter === 'resolved' ? styles.filterBtnActive : ''}`}
                            onClick={() => setFilter('resolved')}
                        >
                            已解决
                        </button>
                    </div>
                </div>

                {loading && (
                    <LoadingSpinner />
                )}

                {!loading && data && (
                    <>
                        {/* Summary Cards */}
                        <div className={styles.summaryRow}>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>总错题数</div>
                                <div className={styles.summaryValue}>
                                    {data.total_count}
                                    <span className={styles.summaryUnit}>条</span>
                                </div>
                            </div>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>待解决</div>
                                <div className={styles.summaryValue}>
                                    {data.unresolved_count}
                                    <span className={styles.summaryUnit}>条</span>
                                </div>
                            </div>
                            <div className={styles.summaryCard}>
                                <div className={styles.summaryLabel}>已解决</div>
                                <div className={styles.summaryValue}>
                                    {data.resolved_count}
                                    <span className={styles.summaryUnit}>条</span>
                                </div>
                            </div>
                        </div>

                        {/* Entry List */}
                        {data.items.length > 0 ? (
                            <div className={styles.entryList}>
                                {data.items.map((item: ErrorNotebookItem) => {
                                    const isExpanded = expandedIds.has(item.id);
                                    return (
                                        <div key={item.id} className={styles.entryCard}>
                                            <div className={styles.entryHeader}>
                                                <div>
                                                    <div className={styles.entryKP}>{item.kp_title}</div>
                                                    <div className={styles.entryChapter}>{item.chapter_title}</div>
                                                </div>
                                                <div className={styles.entryBadges}>
                                                    <span className={styles.badgeMastery}>
                                                        {Math.round(item.mastery_at_error * 100)}%
                                                    </span>
                                                    {item.resolved ? (
                                                        <span className={styles.badgeResolved}>已解决</span>
                                                    ) : (
                                                        <span className={styles.badgeUnresolved}>待解决</span>
                                                    )}
                                                </div>
                                            </div>

                                            <div className={styles.conversation}>
                                                <div className={`${styles.msgBlock} ${styles.msgStudent}`}>
                                                    <div className={`${styles.msgLabel} ${styles.msgLabelStudent}`}>
                                                        我的回答
                                                    </div>
                                                    {isExpanded
                                                        ? item.student_input
                                                        : truncateText(item.student_input, 150)}
                                                </div>
                                                <div className={`${styles.msgBlock} ${styles.msgCoach}`}>
                                                    <div className={`${styles.msgLabel} ${styles.msgLabelCoach}`}>
                                                        AI 引导
                                                    </div>
                                                    {isExpanded
                                                        ? item.coach_guidance
                                                        : truncateText(item.coach_guidance, 200)}
                                                </div>
                                            </div>

                                            {(item.student_input.length > 150 || item.coach_guidance.length > 200) && (
                                                <button
                                                    className={styles.expandBtn}
                                                    onClick={() => toggleExpand(item.id)}
                                                >
                                                    {isExpanded ? '收起' : '展开完整内容'}
                                                </button>
                                            )}

                                            <div className={styles.entryMeta}>
                                                <span>
                                                    归档时间{' '}
                                                    <span className={styles.metaValue}>
                                                        {new Date(item.archived_at).toLocaleDateString('zh-CN')}
                                                    </span>
                                                </span>
                                                {item.resolved && item.resolved_at && (
                                                    <span>
                                                        解决时间{' '}
                                                        <span className={styles.metaValue}>
                                                            {new Date(item.resolved_at).toLocaleDateString('zh-CN')}
                                                        </span>
                                                    </span>
                                                )}
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        ) : (
                            <div className={styles.emptyState}>
                                <div className={styles.emptyIcon}>
                                    {filter === 'resolved' ? '🎉' : '📝'}
                                </div>
                                <div className={styles.emptyText}>
                                    {filter === 'all'
                                        ? '暂无错题记录，继续学习中会自动归档'
                                        : filter === 'unresolved'
                                          ? '没有待解决的错题，太棒了！'
                                          : '暂无已解决的错题'}
                                </div>
                            </div>
                        )}
                    </>
                )}

                {!loading && !data && (
                    <div className={styles.emptyState}>
                        <div className={styles.emptyIcon}>⚠️</div>
                        <div className={styles.emptyText}>加载错题本失败，请稍后重试</div>
                    </div>
                )}
            </div>
    );
}
