'use client';

import { useEffect, useState } from 'react';
import dynamic from 'next/dynamic';
import { getSelfMastery, type StudentMasteryData } from '@/lib/api';
import styles from './page.module.css';

const MasteryTrendChart = dynamic(() => import('./MasteryTrendChart'), { ssr: false });

export default function StudentMasteryPage() {
    const [data, setData] = useState<StudentMasteryData | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        getSelfMastery()
            .then(setData)
            .catch(console.error)
            .finally(() => setLoading(false));
    }, []);

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
        );
    }

    if (!data || data.items.length === 0) {
        return (
            <div className="fade-in">
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>我的掌握度</h1>
                </div>
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>📈</div>
                    <div className={styles.emptyText}>暂无学习数据，完成学习活动后这里将展示你的掌握度</div>
                </div>
            </div>
        );
    }

    // Calculate summary stats
    const totalKPs = data.items.length;
    const avgMastery = data.items.reduce((s, i) => s + i.mastery_score, 0) / totalKPs;
    const masteredKPs = data.items.filter(i => i.mastery_score >= 0.8).length;
    const totalAttempts = data.items.reduce((s, i) => s + i.attempt_count, 0);

    // Mastery class helper
    const getMasteryClass = (score: number) => {
        if (score >= 0.8) return { badge: styles.masteryHigh, bar: styles.progressHigh };
        if (score >= 0.5) return { badge: styles.masteryMedium, bar: styles.progressMedium };
        return { badge: styles.masteryLow, bar: styles.progressLow };
    };

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>我的掌握度</h1>
            </div>

            {/* Summary Cards */}
            <div className={styles.summaryRow}>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>涉及知识点</div>
                    <div className={styles.summaryValue}>
                        {totalKPs}
                        <span className={styles.summaryUnit}>个</span>
                    </div>
                </div>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>平均掌握度</div>
                    <div className={styles.summaryValue}>
                        {Math.round(avgMastery * 100)}
                        <span className={styles.summaryUnit}>%</span>
                    </div>
                </div>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>已掌握</div>
                    <div className={styles.summaryValue}>
                        {masteredKPs}
                        <span className={styles.summaryUnit}>/ {totalKPs}</span>
                    </div>
                </div>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>练习次数</div>
                    <div className={styles.summaryValue}>
                        {totalAttempts}
                        <span className={styles.summaryUnit}>次</span>
                    </div>
                </div>
            </div>

            {/* Trend Chart */}
            {data.history.length > 0 && (
                <div className={styles.chartCard}>
                    <div className={styles.chartTitle}>掌握度变化趋势</div>
                    <MasteryTrendChart
                        dates={data.history.map(h => h.date)}
                        values={data.history.map(h => h.avg_mastery)}
                    />
                </div>
            )}

            {/* Knowledge Point Mastery Cards */}
            <h2 className={styles.sectionTitle}>各知识点掌握详情</h2>
            <div className={styles.masteryGrid}>
                {data.items
                    .sort((a, b) => a.mastery_score - b.mastery_score)  // weakest first
                    .map(item => {
                        const pct = Math.round(item.mastery_score * 100);
                        const cls = getMasteryClass(item.mastery_score);
                        return (
                            <div key={item.kp_id} className={styles.masteryCard}>
                                <div className={styles.masteryCardHeader}>
                                    <div>
                                        <div className={styles.kpTitle}>{item.kp_title}</div>
                                        <div className={styles.chapterName}>{item.chapter_title}</div>
                                    </div>
                                    <span className={`${styles.masteryBadge} ${cls.badge}`}>
                                        {pct}%
                                    </span>
                                </div>
                                <div className={styles.progressBarWrap}>
                                    <div
                                        className={`${styles.progressBar} ${cls.bar}`}
                                        style={{ width: `${pct}%` }}
                                    />
                                </div>
                                <div className={styles.masteryMeta}>
                                    <span>
                                        练习 <span className={styles.metaValue}>{item.attempt_count}</span> 次
                                    </span>
                                    <span>
                                        正确 <span className={styles.metaValue}>{item.correct_count}</span> 次
                                    </span>
                                    {item.last_attempt_at && (
                                        <span>
                                            最近 <span className={styles.metaValue}>
                                                {new Date(item.last_attempt_at).toLocaleDateString('zh-CN')}
                                            </span>
                                        </span>
                                    )}
                                </div>
                            </div>
                        );
                    })}
            </div>
        </div>
    );
}
