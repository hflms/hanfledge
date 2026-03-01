'use client';

import { useEffect, useState } from 'react';
import { useRouter } from 'next/navigation';
import { listStudentActivities, joinActivity, type LearningActivity } from '@/lib/api';
import { useToast } from '@/components/Toast';
import { SESSION_STATUS_MAP } from '@/lib/constants';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

export default function StudentActivitiesPage() {
    const router = useRouter();
    const { toast } = useToast();
    const [activities, setActivities] = useState<LearningActivity[]>([]);
    const [loading, setLoading] = useState(true);
    const [joining, setJoining] = useState<number | null>(null);

    const fetchActivities = async () => {
        try {
            const data = await listStudentActivities();
            setActivities(data);
        } catch (err) {
            console.error('Failed to load activities:', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchActivities();
    }, []);

    const handleJoin = async (activityId: number) => {
        setJoining(activityId);
        try {
            const result = await joinActivity(activityId);
            router.push(`/student/session/${result.session_id}`);
        } catch (err) {
            console.error('Failed to join activity:', err);
            toast('加入活动失败，请重试', 'error');
        } finally {
            setJoining(null);
        }
    };

    const handleContinue = (sessionId: number) => {
        router.push(`/student/session/${sessionId}`);
    };

    if (loading) {
        return (
            <LoadingSpinner size="large" />
        );
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>学习活动</h1>
            </div>

            {activities.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>📋</div>
                    <div className={styles.emptyText}>暂无可用的学习活动</div>
                </div>
            ) : (
                <div className={styles.activityGrid}>
                    {activities.map(activity => (
                        <div key={activity.id} className={`card ${styles.activityCard}`}>
                            <div className={styles.cardHeader}>
                                <div>
                                    <div className={styles.activityTitle}>{activity.title}</div>
                                </div>
                                {activity.has_session ? (
                                    <span className={`${styles.statusBadge} ${
                                        activity.session_status === 'completed'
                                            ? styles.statusCompleted
                                            : styles.statusActive
                                    }`}>
                                        {SESSION_STATUS_MAP[activity.session_status || ''] || '学习中'}
                                    </span>
                                ) : (
                                    <span className={`${styles.statusBadge} ${styles.statusNew}`}>
                                        未开始
                                    </span>
                                )}
                            </div>

                            <div className={styles.activityMeta}>
                                {activity.deadline && (
                                    <div className={styles.metaItem}>
                                        截止日期 <span className={styles.metaValue}>
                                            {new Date(activity.deadline).toLocaleDateString('zh-CN')}
                                        </span>
                                    </div>
                                )}
                                <div className={styles.metaItem}>
                                    最大尝试 <span className={styles.metaValue}>{activity.max_attempts} 次</span>
                                </div>
                                <div className={styles.metaItem}>
                                    允许重试 <span className={styles.metaValue}>{activity.allow_retry ? '是' : '否'}</span>
                                </div>
                            </div>

                            <div className={styles.cardActions}>
                                {activity.has_session && activity.session_id ? (
                                    activity.session_status === 'active' ? (
                                        <button
                                            className="btn btn-primary"
                                            onClick={() => handleContinue(activity.session_id!)}
                                        >
                                            继续学习
                                        </button>
                                    ) : (
                                        <button className="btn btn-secondary" disabled>
                                            已完成
                                        </button>
                                    )
                                ) : (
                                    <button
                                        className="btn btn-primary"
                                        onClick={() => handleJoin(activity.id)}
                                        disabled={joining === activity.id}
                                    >
                                        {joining === activity.id ? '加入中...' : '开始学习'}
                                    </button>
                                )}
                            </div>
                        </div>
                    ))}
                </div>
            )}
        </div>
    );
}
