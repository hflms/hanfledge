'use client';

import { useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import dynamic from 'next/dynamic';
import {
    exportClassMastery,
    exportErrorNotebookCSV,
    exportActivitySessions,
    exportInteractionLog,
    previewActivity,
    getActivitySessions,
    type Course,
    type LearningActivity,
    type KnowledgeRadarData,
    type ActivitySessionStats,
    type SkillEffectivenessResponse,
    type PaginatedResponse,
} from '@/lib/api';
import { useApi } from '@/lib/useApi';
import styles from './page.module.css';
import PluginSlot from '@/components/PluginSlot';
import { useBuiltinDashboardPlugins } from '@/lib/plugin/DashboardPlugins';
import { useToast } from '@/components/Toast';
import { ACTIVITY_STATUS_MAP, SCAFFOLD_MAP } from '@/lib/constants';
import { useModalA11y } from '@/lib/a11y';
import LoadingSpinner from '@/components/LoadingSpinner';

const RadarChart = dynamic(() => import('./RadarChart'), { ssr: false });
const MasteryBarChart = dynamic(() => import('./MasteryBarChart'), { ssr: false });
const SkillEffectivenessChart = dynamic(() => import('./SkillEffectivenessChart'), { ssr: false });

// -- Main Component ---------------------------------------

export default function TeacherDashboardPage() {
    const router = useRouter();
    const { toast } = useToast();
    const { data: coursesData, isLoading: loading } = useApi<PaginatedResponse<Course>>('/courses');
    const courses = coursesData?.items || [];
    
    const [selectedCourseId, setSelectedCourseId] = useState<number | null>(null);
    const [selectedActivity, setSelectedActivity] = useState<ActivitySessionStats | null>(null);
    const closeActivityModal = useCallback(() => setSelectedActivity(null), []);
    const activityModalRef = useModalA11y(!!selectedActivity, closeActivityModal);
    const [exporting, setExporting] = useState<string | null>(null);

    // Register built-in dashboard widget plugins
    useBuiltinDashboardPlugins();

    // Derive effective course ID synchronously to avoid useEffect waterfall
    const effectiveCourseId = selectedCourseId ?? (courses.length > 0 ? courses[0].id : null);

    // Data fetching via SWR handles caching, deduplication and avoids waterfalls
    const { data: radarData, isLoading: radarLoading } = useApi<KnowledgeRadarData>(
        effectiveCourseId ? `/dashboard/knowledge-radar?course_id=${effectiveCourseId}` : null
    );

    const { data: activitiesData } = useApi<PaginatedResponse<LearningActivity>>(
        effectiveCourseId ? `/activities?course_id=${effectiveCourseId}` : null
    );
    const activities = activitiesData?.items || [];

    const { data: skillEffectiveness } = useApi<SkillEffectivenessResponse>(
        effectiveCourseId ? `/dashboard/skill-effectiveness?course_id=${effectiveCourseId}` : null
    );

    // Handle activity click to show session details
    const handleActivityClick = async (activityId: number) => {
        try {
            const stats = await getActivitySessions(activityId);
            setSelectedActivity(stats);
        } catch (err) {
            console.error('Failed to load activity sessions', err);
        }
    };

    // Handle CSV export with loading state
    const handleExport = async (type: string, fn: () => Promise<void>) => {
        setExporting(type);
        try {
            await fn();
        } catch (err) {
            console.error(`Export ${type} failed`, err);
            toast(`导出失败: ${err instanceof Error ? err.message : '未知错误'}`, 'error');
        } finally {
            setExporting(null);
        }
    };

    // Handle sandbox preview — creates sandbox session and navigates to it
    const handlePreview = async (activityId: number) => {
        try {
            const result = await previewActivity(activityId);
            router.push(`/student/session/${result.session_id}`);
        } catch (err) {
            console.error('Preview failed', err);
            toast(`预览失败: ${err instanceof Error ? err.message : '未知错误'}`, 'error');
        }
    };

    if (loading) {
        return (
            <LoadingSpinner />
        );
    }

    if (courses.length === 0) {
        return (
            <div className="fade-in">
                <div className={styles.pageHeader}>
                    <h1 className={styles.pageTitle}>学情仪表盘</h1>
                </div>
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>📊</div>
                    <div className={styles.emptyText}>还没有课程数据，请先创建课程并发布学习活动</div>
                </div>
            </div>
        );
    }

    const publishedActivities = activities.filter(a => a.status === 'published');

    return (
        <div className="fade-in">
            {/* Page Header */}
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>学情仪表盘</h1>
                <div className={styles.controls}>
                    <select
                        className={styles.select}
                        value={effectiveCourseId || ''}
                        onChange={e => setSelectedCourseId(Number(e.target.value))}
                    >
                        {courses.map(c => (
                            <option key={c.id} value={c.id}>{c.title}</option>
                        ))}
                    </select>
                    <div className={styles.exportGroup}>
                        <button
                            className={styles.exportBtn}
                            disabled={!!exporting || !effectiveCourseId}
                            onClick={() => effectiveCourseId && handleExport('mastery', () => exportClassMastery(effectiveCourseId))}
                        >
                            {exporting === 'mastery' ? '导出中...' : '导出掌握度'}
                        </button>
                        <button
                            className={styles.exportBtn}
                            disabled={!!exporting || !effectiveCourseId}
                            onClick={() => effectiveCourseId && handleExport('errors', () => exportErrorNotebookCSV(effectiveCourseId))}
                        >
                            {exporting === 'errors' ? '导出中...' : '导出错题本'}
                        </button>
                    </div>
                </div>
            </div>

            {/* Stats Overview Cards */}
            <div className={styles.statsRow}>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>知识点数量</div>
                    <div className={styles.statValue}>
                        {radarData?.labels.length || 0}
                    </div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>学习学生数</div>
                    <div className={styles.statValue}>
                        {radarData?.student_count || 0}
                        <span className={styles.statUnit}>人</span>
                    </div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>学习活动</div>
                    <div className={styles.statValue}>
                        {publishedActivities.length}
                        <span className={styles.statUnit}>个</span>
                    </div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>平均掌握度</div>
                    <div className={styles.statValue}>
                        {radarData && radarData.values.length > 0
                            ? Math.round(
                                (radarData.values.reduce((a, b) => a + b, 0) / radarData.values.length) * 100
                            )
                            : 0}
                        <span className={styles.statUnit}>%</span>
                    </div>
                </div>
            </div>

            {/* Charts */}
            {radarLoading ? (
                <LoadingSpinner size="small" />
            ) : radarData && radarData.labels.length > 0 ? (
                <div className={styles.chartsGrid}>
                    <div className={styles.chartCard}>
                        <div className={styles.chartTitle}>全班知识掌握雷达</div>
                        <RadarChart labels={radarData.labels} values={radarData.values} />
                    </div>
                    <div className={styles.chartCard}>
                        <div className={styles.chartTitle}>各知识点平均掌握度</div>
                        <MasteryBarChart labels={radarData.labels} values={radarData.values} />
                    </div>
                </div>
            ) : (
                <div className={styles.chartsGrid}>
                    <div className={styles.chartCard}>
                        <div className={styles.chartTitle}>全班知识掌握雷达</div>
                        <div className={styles.emptyState}>
                            <div className={styles.emptyText}>暂无学习数据</div>
                        </div>
                    </div>
                    <div className={styles.chartCard}>
                        <div className={styles.chartTitle}>各知识点平均掌握度</div>
                        <div className={styles.emptyState}>
                            <div className={styles.emptyText}>暂无学习数据</div>
                        </div>
                    </div>
                </div>
            )}

            {/* Skill Effectiveness */}
            {skillEffectiveness && skillEffectiveness.items.length > 0 && (
                <div className={styles.skillEffectivenessSection}>
                    <div className={styles.chartCard}>
                        <div className={styles.chartTitle}>技能教学效果评估 (RAGAS)</div>
                        <SkillEffectivenessChart items={skillEffectiveness.items} />
                    </div>
                </div>
            )}

            {/* Plugin Extension Slot — additional dashboard widgets render here */}
            <PluginSlot
                name="teacher.dashboard.widget"
                context={{
                    courseId: effectiveCourseId || 0,
                    courseTitle: courses.find(c => c.id === effectiveCourseId)?.title || '',
                }}
            />

            {/* Activity Table */}
            <h2 className={styles.sectionTitle}>学习活动统计</h2>
            {activities.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyText}>该课程暂无学习活动</div>
                </div>
            ) : (
                <table className={styles.activityTable}>
                    <thead>
                        <tr>
                            <th>活动名称</th>
                            <th>状态</th>
                            <th>创建时间</th>
                            <th>操作</th>
                        </tr>
                    </thead>
                    <tbody>
                        {activities.map(act => (
                            <tr key={act.id} className={styles.activityRow}>
                                <td className={styles.activityTitle}>{act.title}</td>
                                <td>
                                    <span className={`badge badge-${act.status}`}>
                                        {ACTIVITY_STATUS_MAP[act.status] || act.status}
                                    </span>
                                </td>
                                <td>{new Date(act.created_at).toLocaleDateString('zh-CN')}</td>
                                <td>
                                    <button
                                        className="btn btn-ghost btn-sm"
                                        onClick={() => handlePreview(act.id)}
                                        title="以学生视角预览活动"
                                    >
                                        预览
                                    </button>
                                    <button
                                        className="btn btn-ghost btn-sm"
                                        onClick={() => handleActivityClick(act.id)}
                                    >
                                        查看详情
                                    </button>
                                </td>
                            </tr>
                        ))}
                    </tbody>
                </table>
            )}

            {/* Activity Sessions Modal */}
            {selectedActivity && (
                <div className={styles.modalOverlay} onClick={closeActivityModal}>
                    <div className={styles.modal} onClick={e => e.stopPropagation()}
                         ref={activityModalRef} role="dialog" aria-modal="true" aria-labelledby="activity-modal-title" tabIndex={-1}>
                        <h2 className={styles.modalTitle} id="activity-modal-title">{selectedActivity.activity_title} — 会话统计</h2>

                        <div className={styles.statsRow}>
                            <div className={styles.statCard}>
                                <div className={styles.statLabel}>总会话数</div>
                                <div className={styles.statValue}>{selectedActivity.total_sessions}</div>
                            </div>
                            <div className={styles.statCard}>
                                <div className={styles.statLabel}>完成率</div>
                                <div className={styles.statValue}>
                                    {Math.round(selectedActivity.completion_rate)}
                                    <span className={styles.statUnit}>%</span>
                                </div>
                            </div>
                            <div className={styles.statCard}>
                                <div className={styles.statLabel}>平均时长</div>
                                <div className={styles.statValue}>
                                    {Math.round(selectedActivity.avg_duration_min)}
                                    <span className={styles.statUnit}>分</span>
                                </div>
                            </div>
                            <div className={styles.statCard}>
                                <div className={styles.statLabel}>平均掌握度</div>
                                <div className={styles.statValue}>
                                    {Math.round(selectedActivity.avg_mastery * 100)}
                                    <span className={styles.statUnit}>%</span>
                                </div>
                            </div>
                        </div>

                        {selectedActivity.sessions.length > 0 ? (
                            <table className={styles.sessionTable}>
                                <thead>
                                     <tr>
                                        <th>学生</th>
                                        <th>状态</th>
                                        <th>支架</th>
                                        <th>掌握度</th>
                                        <th>时长</th>
                                        <th>开始时间</th>
                                        <th>操作</th>
                                    </tr>
                                </thead>
                                <tbody>
                                    {selectedActivity.sessions.map(s => {
                                        const masteryPct = Math.round(s.mastery_score * 100);
                                        const masteryClass = masteryPct >= 80
                                            ? styles.masteryHigh
                                            : masteryPct >= 50
                                                ? styles.masteryMedium
                                                : styles.masteryLow;
                                        return (
                                            <tr key={s.session_id}>
                                                <td>{s.student_name}</td>
                                                <td>
                                                    <span className={`badge badge-${s.status}`}>
                                                        {ACTIVITY_STATUS_MAP[s.status] || s.status}
                                                    </span>
                                                </td>
                                                <td>{SCAFFOLD_MAP[s.scaffold_level] || s.scaffold_level}</td>
                                                <td>
                                                    <span className={`${styles.masteryBadge} ${masteryClass}`}>
                                                        {masteryPct}%
                                                    </span>
                                                </td>
                                                <td>{Math.round(s.duration_min)} 分钟</td>
                                                <td>{new Date(s.started_at).toLocaleString('zh-CN')}</td>
                                                <td>
                                                    <button
                                                        className={styles.analysisBtn}
                                                        onClick={(e) => {
                                                            e.stopPropagation();
                                                            router.push(`/teacher/dashboard/session/${s.session_id}`);
                                                        }}
                                                    >
                                                        查看分析
                                                    </button>
                                                    <button
                                                        className={styles.exportSmallBtn}
                                                        disabled={!!exporting}
                                                        onClick={(e) => {
                                                            e.stopPropagation();
                                                            handleExport(`interaction-${s.session_id}`, () => exportInteractionLog(s.session_id));
                                                        }}
                                                    >
                                                        导出
                                                    </button>
                                                </td>
                                            </tr>
                                        );
                                    })}
                                </tbody>
                            </table>
                        ) : (
                            <div className={styles.emptyState}>
                                <div className={styles.emptyText}>暂无学习会话</div>
                            </div>
                        )}

                        <div className={styles.modalClose}>
                            <button
                                className={styles.exportBtn}
                                disabled={!!exporting}
                                onClick={() => handleExport('sessions', () => exportActivitySessions(selectedActivity.activity_id))}
                            >
                                {exporting === 'sessions' ? '导出中...' : '导出会话 CSV'}
                            </button>
                            <button className="btn btn-secondary" onClick={closeActivityModal}>
                                关闭
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
