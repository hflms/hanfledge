'use client';

import { useEffect, useState, useCallback } from 'react';
import { useRouter } from 'next/navigation';
import {
    listCourses,
    listTeacherActivities,
    getKnowledgeRadar,
    getActivitySessions,
    getSkillEffectiveness,
    exportClassMastery,
    exportErrorNotebookCSV,
    exportActivitySessions,
    exportInteractionLog,
    type Course,
    type LearningActivity,
    type KnowledgeRadarData,
    type ActivitySessionStats,
    type SkillEffectivenessResponse,
} from '@/lib/api';
import styles from './page.module.css';
import RadarChart from './RadarChart';
import MasteryBarChart from './MasteryBarChart';
import SkillEffectivenessChart from './SkillEffectivenessChart';
import PluginSlot from '@/components/PluginSlot';
import { useBuiltinDashboardPlugins } from '@/lib/plugin/DashboardPlugins';

// -- Status Maps ------------------------------------------

const STATUS_MAP: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    closed: '已关闭',
    active: '进行中',
    completed: '已完成',
    abandoned: '已放弃',
};

const SCAFFOLD_MAP: Record<string, string> = {
    high: '高',
    medium: '中',
    low: '低',
};

// -- Main Component ---------------------------------------

export default function TeacherDashboardPage() {
    const router = useRouter();
    const [courses, setCourses] = useState<Course[]>([]);
    const [selectedCourseId, setSelectedCourseId] = useState<number | null>(null);
    const [radarData, setRadarData] = useState<KnowledgeRadarData | null>(null);
    const [activities, setActivities] = useState<LearningActivity[]>([]);
    const [selectedActivity, setSelectedActivity] = useState<ActivitySessionStats | null>(null);
    const [skillEffectiveness, setSkillEffectiveness] = useState<SkillEffectivenessResponse | null>(null);
    const [loading, setLoading] = useState(true);
    const [radarLoading, setRadarLoading] = useState(false);
    const [exporting, setExporting] = useState<string | null>(null);

    // Register built-in dashboard widget plugins
    useBuiltinDashboardPlugins();

    // Load courses on mount
    useEffect(() => {
        listCourses()
            .then(data => {
                const list = data || [];
                setCourses(list);
                if (list.length > 0) {
                    setSelectedCourseId(list[0].id);
                }
            })
            .catch(console.error)
            .finally(() => setLoading(false));
    }, []);

    // Load radar data + activities when course changes
    const loadDashboardData = useCallback(async (courseId: number) => {
        setRadarLoading(true);
        try {
            const [radar, acts, skillData] = await Promise.all([
                getKnowledgeRadar(courseId),
                listTeacherActivities(courseId),
                getSkillEffectiveness(courseId).catch(() => null),
            ]);
            setRadarData(radar);
            setActivities(acts || []);
            setSkillEffectiveness(skillData);
        } catch (err) {
            console.error('Failed to load dashboard data', err);
        } finally {
            setRadarLoading(false);
        }
    }, []);

    useEffect(() => {
        if (selectedCourseId) {
            loadDashboardData(selectedCourseId);
        }
    }, [selectedCourseId, loadDashboardData]);

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
            alert(`导出失败: ${err instanceof Error ? err.message : '未知错误'}`);
        } finally {
            setExporting(null);
        }
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
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
                        value={selectedCourseId || ''}
                        onChange={e => setSelectedCourseId(Number(e.target.value))}
                    >
                        {courses.map(c => (
                            <option key={c.id} value={c.id}>{c.title}</option>
                        ))}
                    </select>
                    <div className={styles.exportGroup}>
                        <button
                            className={styles.exportBtn}
                            disabled={!!exporting || !selectedCourseId}
                            onClick={() => selectedCourseId && handleExport('mastery', () => exportClassMastery(selectedCourseId))}
                        >
                            {exporting === 'mastery' ? '导出中...' : '导出掌握度'}
                        </button>
                        <button
                            className={styles.exportBtn}
                            disabled={!!exporting || !selectedCourseId}
                            onClick={() => selectedCourseId && handleExport('errors', () => exportErrorNotebookCSV(selectedCourseId))}
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
                <div style={{ display: 'flex', justifyContent: 'center', padding: 40 }}>
                    <div className="spinner" />
                </div>
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
                    courseId: selectedCourseId || 0,
                    courseTitle: courses.find(c => c.id === selectedCourseId)?.title || '',
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
                                        {STATUS_MAP[act.status] || act.status}
                                    </span>
                                </td>
                                <td>{new Date(act.created_at).toLocaleDateString('zh-CN')}</td>
                                <td>
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
                <div className={styles.modalOverlay} onClick={() => setSelectedActivity(null)}>
                    <div className={styles.modal} onClick={e => e.stopPropagation()}>
                        <h2 className={styles.modalTitle}>{selectedActivity.activity_title} — 会话统计</h2>

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
                                                        {STATUS_MAP[s.status] || s.status}
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
                            <button className="btn btn-secondary" onClick={() => setSelectedActivity(null)}>
                                关闭
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
