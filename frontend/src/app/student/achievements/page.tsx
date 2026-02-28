'use client';

import { useEffect, useState } from 'react';
import { getMyAchievements, type StudentAchievementsData, type AchievementProgress } from '@/lib/api';
import styles from './page.module.css';

// -- Achievement Type Labels ----------------------------------------

const TYPE_LABELS: Record<string, string> = {
    streak_breaker: '连续突破',
    deep_inquiry: '深度追问',
    fallacy_hunter: '谬误猎人',
};

const TYPE_DESCRIPTIONS: Record<string, string> = {
    streak_breaker: '连续掌握知识点，展示学习势头',
    deep_inquiry: '单次会话深度追问，培养批判性思维',
    fallacy_hunter: '识别 AI 嵌入的谬误，锤炼辨析能力',
};

const TIER_LABELS: Record<string, string> = {
    bronze: '铜牌',
    silver: '银牌',
    gold: '金牌',
    diamond: '钻石',
};

// -- Helpers ----------------------------------------

function groupByType(achievements: AchievementProgress[]): Record<string, AchievementProgress[]> {
    const groups: Record<string, AchievementProgress[]> = {};
    for (const a of achievements) {
        if (!groups[a.type]) groups[a.type] = [];
        groups[a.type].push(a);
    }
    return groups;
}

// -- Component ----------------------------------------

export default function StudentAchievementsPage() {
    const [data, setData] = useState<StudentAchievementsData | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        getMyAchievements()
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

    if (!data || data.achievements.length === 0) {
        return (
            <div className="fade-in">
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>我的成就</h1>
                </div>
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>🏆</div>
                    <div className={styles.emptyText}>成就系统初始化中，开始学习后将自动解锁成就</div>
                </div>
            </div>
        );
    }

    const groups = groupByType(data.achievements);
    const typeOrder = ['streak_breaker', 'deep_inquiry', 'fallacy_hunter'];

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>我的成就</h1>
            </div>

            {/* Summary Cards */}
            <div className={styles.summaryRow}>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>已解锁</div>
                    <div className={styles.summaryValue}>
                        {data.total_unlocked}
                        <span className={styles.summaryUnit}>/ {data.total_count}</span>
                    </div>
                </div>
                <div className={styles.summaryCard}>
                    <div className={styles.summaryLabel}>完成率</div>
                    <div className={styles.summaryValue}>
                        {data.total_count > 0 ? Math.round((data.total_unlocked / data.total_count) * 100) : 0}
                        <span className={styles.summaryUnit}>%</span>
                    </div>
                </div>
                {typeOrder.map(type => {
                    const typeAchievements = groups[type] || [];
                    const unlocked = typeAchievements.filter(a => a.unlocked).length;
                    return (
                        <div key={type} className={styles.summaryCard}>
                            <div className={styles.summaryLabel}>{TYPE_LABELS[type]}</div>
                            <div className={styles.summaryValue}>
                                {unlocked}
                                <span className={styles.summaryUnit}>/ {typeAchievements.length}</span>
                            </div>
                        </div>
                    );
                })}
            </div>

            {/* Achievement Groups */}
            {typeOrder.map(type => {
                const typeAchievements = groups[type];
                if (!typeAchievements) return null;

                return (
                    <div key={type} className={styles.typeSection}>
                        <div className={styles.typeHeader}>
                            <h2 className={styles.sectionTitle}>{TYPE_LABELS[type]}</h2>
                            <span className={styles.typeDescription}>
                                {TYPE_DESCRIPTIONS[type]}
                            </span>
                        </div>

                        <div className={styles.achievementGrid}>
                            {typeAchievements.map(achievement => (
                                <AchievementCard key={achievement.id} achievement={achievement} />
                            ))}
                        </div>
                    </div>
                );
            })}
        </div>
    );
}

// -- Achievement Card ----------------------------------------

function AchievementCard({ achievement }: { achievement: AchievementProgress }) {
    const pct = achievement.threshold > 0
        ? Math.min(Math.round((achievement.progress / achievement.threshold) * 100), 100)
        : 0;

    const tierClass = styles[`tier${achievement.tier.charAt(0).toUpperCase()}${achievement.tier.slice(1)}`] || '';

    return (
        <div className={`${styles.achievementCard} ${achievement.unlocked ? styles.unlocked : styles.locked}`}>
            <div className={styles.cardIcon}>{achievement.icon}</div>
            <div className={styles.cardContent}>
                <div className={styles.cardHeader}>
                    <span className={styles.cardName}>{achievement.name}</span>
                    <span className={`${styles.tierBadge} ${tierClass}`}>
                        {TIER_LABELS[achievement.tier]}
                    </span>
                </div>
                <div className={styles.cardDescription}>{achievement.description}</div>
                <div className={styles.progressWrap}>
                    <div className={styles.progressBarOuter}>
                        <div
                            className={`${styles.progressBarInner} ${achievement.unlocked ? styles.progressComplete : ''}`}
                            style={{ width: `${pct}%` }}
                        />
                    </div>
                    <span className={styles.progressText}>
                        {achievement.progress} / {achievement.threshold}
                    </span>
                </div>
                {achievement.unlocked && achievement.unlocked_at && (
                    <div className={styles.unlockedDate}>
                        {new Date(achievement.unlocked_at).toLocaleDateString('zh-CN')} 解锁
                    </div>
                )}
            </div>
        </div>
    );
}
