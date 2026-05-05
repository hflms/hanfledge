'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import {
    getCourseOutline, uploadMaterial, getDocuments,
    listSkills, mountSkill,
    mountSkillToKP, unmountSkillFromKP, updateKPSkillConfig,
    recommendSkills, batchMountSkills,
    listTeacherActivities, createActivity, publishActivity,
    type Course, type Document, type SkillMetadata, type MountedSkill, type RecommendMount, type LearningActivity
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import { DOCUMENT_STATUS_LABEL, CATEGORY_ICONS } from '@/lib/constants';
import { useModalA11y, handleCardKeyDown } from '@/lib/a11y';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

export default function OutlinePage() {
    const params = useParams();
    const router = useRouter();
    const courseId = Number(params.id);
    const { toast } = useToast();
    const fileInput = useRef<HTMLInputElement>(null);

    const [course, setCourse] = useState<Course | null>(null);
    const [docs, setDocs] = useState<Document[]>([]);
    const [loading, setLoading] = useState(true);
    const [uploading, setUploading] = useState(false);
    const [dragging, setDragging] = useState(false);

    // -- Skill mounting state ----------------------------------------
    const [pickerChapterId, setPickerChapterId] = useState<number | null>(null);
    const [pickerKpId, setPickerKpId] = useState<number | null>(null);
    const [availableSkills, setAvailableSkills] = useState<SkillMetadata[]>([]);
    const [skillsLoading, setSkillsLoading] = useState(false);
    const [mounting, setMounting] = useState(false);
    const [unmounting, setUnmounting] = useState<number | null>(null);
    const pickerRef = useRef<HTMLDivElement>(null);

    // -- Skill config panel state ------------------------------------
    const [configMount, setConfigMount] = useState<{ mount: MountedSkill; chapterId: number; kpId: number } | null>(null);
    const closeConfigModal = useCallback(() => setConfigMount(null), []);
    const configModalRef = useModalA11y(!!configMount, closeConfigModal);
    const [configLevel, setConfigLevel] = useState<string>('high');
    const [configThreshold, setConfigThreshold] = useState<string>('');
    const [configDegradeTo, setConfigDegradeTo] = useState<string>('');
    const [configSaving, setConfigSaving] = useState(false);

    // -- AI Auto-Mount state -----------------------------------------
    const [recommending, setRecommending] = useState(false);
    const [recommendations, setRecommendations] = useState<RecommendMount[] | null>(null);
    const [selectedMounts, setSelectedMounts] = useState<Set<string>>(new Set());
    const [batchMounting, setBatchMounting] = useState(false);

    // -- Activity state ----------------------------------------
    const [activities, setActivities] = useState<LearningActivity[]>([]);
    const [creatingActivity, setCreatingActivity] = useState(false);
    const [publishingId, setPublishingId] = useState<number | null>(null);
    const [rightTab, setRightTab] = useState<'materials' | 'activities'>('materials');

    const fetchData = useCallback(async () => {
        if (!courseId || isNaN(courseId)) {
            router.push('/teacher/courses');
            return;
        }
        try {
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setDocs(data.documents || []);
            const activityRes = await listTeacherActivities(courseId, { page: 1, limit: 50 });
            setActivities(activityRes.items || []);
        } catch (err: unknown) {
            const errorMsg = err instanceof Error ? err.message : String(err);
            if (errorMsg.includes('课程不存在')) {
                toast('课程不存在，已返回课程列表', 'error');
                router.push('/teacher/courses');
            } else {
                console.warn('Failed to fetch outline:', err);
                toast('获取课程大纲失败', 'error');
            }
        } finally {
            setLoading(false);
        }
    }, [courseId, router, toast]);

    useEffect(() => {
        fetchData();
    }, [fetchData]);

    // Poll document status while any doc is processing
    useEffect(() => {
        const hasProcessing = docs.some(d => d.status === 'processing' || d.status === 'uploaded');
        if (!hasProcessing) return;

        const interval = setInterval(async () => {
            const freshDocs = await getDocuments(courseId);
            setDocs(freshDocs);
            // If a doc completed, refresh outline too
            if (freshDocs.some(d => d.status === 'completed')) {
                const data = await getCourseOutline(courseId);
                setCourse(data.course);
            }
        }, 3000);

        return () => clearInterval(interval);
    }, [docs, courseId]);

    const handleUpload = useCallback(async (file: File) => {
        if (!file.name.toLowerCase().endsWith('.pdf')) {
            toast('仅支持 PDF 文件', 'warning');
            return;
        }
        setUploading(true);
        try {
            await uploadMaterial(courseId, file);
            const freshDocs = await getDocuments(courseId);
            setDocs(freshDocs);
        } catch (err) {
            console.error('Upload failed', err);
            toast('上传失败', 'error');
        } finally {
            setUploading(false);
        }
    }, [courseId, toast]);

    const handleDrop = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setDragging(false);
        const file = e.dataTransfer.files[0];
        if (file) handleUpload(file);
    }, [handleUpload]);

    // -- Skill picker handlers ----------------------------------------

    const openSkillPicker = async (chapterId: number, e: React.MouseEvent) => {
        e.stopPropagation();
        if (pickerChapterId === chapterId) {
            setPickerChapterId(null);
            return;
        }
        setPickerChapterId(chapterId);
        setPickerKpId(null);
        setSkillsLoading(true);
        try {
            const skills = await listSkills();
            setAvailableSkills(skills || []);
        } catch (err) {
            console.error('Failed to load skills', err);
        } finally {
            setSkillsLoading(false);
        }
    };

    const handleMountSkill = async (chapterId: number, skillId: string) => {
        setMounting(true);
        try {
            await mountSkill(chapterId, { skill_id: skillId });
            // Refresh outline to show new mounts
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setPickerChapterId(null);
        } catch (err) {
            console.error('Failed to mount skill', err);
            toast('挂载技能失败', 'error');
        } finally {
            setMounting(false);
        }
    };

    const openKpSkillPicker = async (kpId: number, e: React.MouseEvent) => {
        e.stopPropagation();
        if (pickerKpId === kpId) {
            setPickerKpId(null);
            return;
        }
        setPickerKpId(kpId);
        setPickerChapterId(null);
        setSkillsLoading(true);
        try {
            const skills = await listSkills();
            setAvailableSkills(skills || []);
        } catch (err) {
            console.error('Failed to load skills', err);
        } finally {
            setSkillsLoading(false);
        }
    };

    const handleMountSkillToKP = async (kpId: number, skillId: string) => {
        setMounting(true);
        try {
            await mountSkillToKP(kpId, { skill_id: skillId });
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setPickerKpId(null);
        } catch (err) {
            console.error('Failed to mount skill to KP', err);
            toast('挂载技能失败', 'error');
        } finally {
            setMounting(false);
        }
    };

    const handleUnmountSkill = async (kpId: number, mountId: number) => {
        if (!confirm('确认卸载该技能？')) return;
        setUnmounting(mountId);
        try {
            await unmountSkillFromKP(kpId, mountId);
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setConfigMount(null);
        } catch (err) {
            console.error('Failed to unmount skill', err);
            toast('卸载技能失败', 'error');
        } finally {
            setUnmounting(null);
        }
    };

    // -- Skill config panel handlers ----------------------------------

    const openConfigPanel = (mount: MountedSkill, chapterId: number, kpId: number, e: React.MouseEvent) => {
        e.stopPropagation();
        setConfigMount({ mount, chapterId, kpId });
        setConfigLevel(mount.scaffold_level || 'high');
        setConfigThreshold(mount.progressive_rule?.mastery_threshold?.toString() || '');
        setConfigDegradeTo(mount.progressive_rule?.degrade_to || '');
    };

    const handleSaveConfig = async () => {
        if (!configMount) return;
        setConfigSaving(true);
        try {
            const progressiveRule: Record<string, unknown> = {};
            const threshold = parseFloat(configThreshold);
            if (!isNaN(threshold) && threshold > 0 && threshold <= 1) {
                progressiveRule.mastery_threshold = threshold;
            }
            if (configDegradeTo) {
                progressiveRule.degrade_to = configDegradeTo;
            }

            await updateKPSkillConfig(configMount.kpId, configMount.mount.id, {
                scaffold_level: configLevel,
                progressive_rule: Object.keys(progressiveRule).length > 0 ? progressiveRule : undefined,
            });
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setConfigMount(null);
        } catch (err) {
            console.error('Failed to save config', err);
            toast('保存配置失败', 'error');
        } finally {
            setConfigSaving(false);
        }
    };

    // -- AI Auto-Mount handlers ---------------------------------------

    const handleRecommend = async () => {
        setRecommending(true);
        try {
            const res = await recommendSkills(courseId);
            setRecommendations(res.recommendations || []);
            // auto select all initially
            const allSet = new Set<string>();
            (res.recommendations || []).forEach((r, i) => allSet.add(i.toString()));
            setSelectedMounts(allSet);
        } catch (err) {
            console.error('Failed to get recommendations', err);
            toast('AI 分析失败，请重试', 'error');
        } finally {
            setRecommending(false);
        }
    };

    const handleBatchMount = async () => {
        if (!recommendations) return;
        const mountsToApply = recommendations
            .filter((_, i) => selectedMounts.has(i.toString()))
            .map(r => ({ kp_id: r.kp_id, skill_id: r.skill_id, scaffold_level: r.scaffold_level }));

        if (mountsToApply.length === 0) {
            toast('请选择至少一项挂载', 'warning');
            return;
        }

        setBatchMounting(true);
        try {
            const res = await batchMountSkills(courseId, mountsToApply);
            toast(`成功批量挂载 ${res.count} 个技能`, 'success');
            setRecommendations(null);
            // Refresh outline
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
        } catch (err) {
            console.error('Batch mount failed', err);
            toast('批量挂载失败', 'error');
        } finally {
            setBatchMounting(false);
        }
    };

    const toggleMountSelection = (indexStr: string) => {
        const newSet = new Set(selectedMounts);
        if (newSet.has(indexStr)) {
            newSet.delete(indexStr);
        } else {
            newSet.add(indexStr);
        }
        setSelectedMounts(newSet);
    };

    // -- Activity handlers -------------------------------------------

    const handleNewActivity = async () => {
        // Collect all KP IDs from course chapters as default
        const allKpIds: number[] = [];
        (course?.chapters || []).forEach((ch) => {
            ch.knowledge_points?.forEach((kp) => allKpIds.push(kp.id));
        });
        if (allKpIds.length === 0) {
            toast('请先上传教材生成知识点后再创建活动', 'warning');
            return;
        }
        setCreatingActivity(true);
        try {
            const activity = await createActivity({
                course_id: courseId,
                title: `${course?.title || '课程'} - 新活动`,
                kp_ids: allKpIds,
            });
            toast('活动草稿已创建，正在跳转设计页...', 'success');
            router.push(`/teacher/activities/${activity.id}/design`);
        } catch (err) {
            console.error('Failed to create activity', err);
            toast('创建活动失败', 'error');
        } finally {
            setCreatingActivity(false);
        }
    };

    const handlePublishActivity = async (activityId: number) => {
        setPublishingId(activityId);
        try {
            await publishActivity(activityId);
            toast('活动已发布', 'success');
            const activityRes = await listTeacherActivities(courseId, { page: 1, limit: 50 });
            setActivities(activityRes.items || []);
        } catch (err) {
            console.error('Failed to publish activity', err);
            toast('发布失败', 'error');
        } finally {
            setPublishingId(null);
        }
    };

    // Close skill picker on outside click
    useEffect(() => {
        if (!pickerChapterId && !pickerKpId) return;
        const handleClickOutside = (e: MouseEvent) => {
            if (pickerRef.current && !pickerRef.current.contains(e.target as Node)) {
                setPickerChapterId(null);
                setPickerKpId(null);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [pickerChapterId, pickerKpId]);

    if (loading) {
        return (
            <LoadingSpinner />
        );
    }

    const chapters = course?.chapters || [];

    return (
        <div className="fade-in">
            <Link href="/teacher/courses" className={styles.backLink}>← 返回课程列表</Link>
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>{course?.title || '课程详情'}</h1>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
                    <Link href={`/teacher/courses/${courseId}/graph`} className="btn btn-secondary">
                        🗺️ 知识图谱
                    </Link>
                    <Link href={`/teacher/courses/${courseId}/materials`} className={styles.materialsLink}>
                        📤 教材管理
                    </Link>
                    <span className={`badge badge-${course?.status}`}>
                        {course?.status === 'draft' ? '草稿' : course?.status}
                    </span>
                </div>
            </div>

            <div className={styles.twoCol}>
                {/* Left: Outline Tree */}
                <div className={styles.outlinePanel}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
                        <h3 style={{ fontSize: 15, fontWeight: 600, margin: 0 }}>📋 课程大纲</h3>
                        {chapters.length > 0 && (
                            <button
                                className={`btn btn-secondary ${styles.autoMountBtn}`}
                                onClick={handleRecommend}
                                disabled={recommending}
                                style={{ padding: '4px 12px', fontSize: 13 }}
                            >
                                {recommending ? '🤖 AI 分析中...' : '🤖 AI 自动挂载技能'}
                            </button>
                        )}
                    </div>

                    {chapters.length === 0 ? (
                        <div className={styles.emptyOutline}>
                            <div style={{ fontSize: 32, marginBottom: 12 }}>📄</div>
                            上传 PDF 教材后，AI 将自动生成知识结构大纲
                        </div>
                    ) : (
                        chapters.map(ch => {
                            // Collect all skill_ids already mounted on any KP in this chapter
                            const mountedSkillIds = new Set<string>();
                            ch.knowledge_points?.forEach(kp => {
                                kp.mounted_skills?.forEach(s => mountedSkillIds.add(s.skill_id));
                            });

                            return (
                                <div key={ch.id} className={styles.chapterNode}>
                                    <div className={styles.chapterHeader}>
                                        <span className={styles.chapterIcon}>📁</span>
                                        <span className={styles.chapterTitle}>{ch.title}</span>
                                        <span className={styles.chapterOrder}>#{ch.sort_order}</span>
                                        <button
                                            className={styles.mountBtn}
                                            title="挂载技能到本章"
                                            onClick={(e) => openSkillPicker(ch.id, e)}
                                        >
                                            {pickerChapterId === ch.id ? '✕' : '+'}
                                        </button>
                                    </div>

                                    {/* Skill Picker Dropdown */}
                                    {pickerChapterId === ch.id && (
                                        <div className={styles.skillPicker} ref={pickerRef}>
                                            <div className={styles.pickerHeader}>选择要挂载的技能</div>
                                            {skillsLoading ? (
                                                <div className={styles.pickerLoading}>
                                                    <div className="spinner" />
                                                </div>
                                            ) : availableSkills.length === 0 ? (
                                                <div className={styles.pickerEmpty}>暂无可用技能</div>
                                            ) : (
                                                <div className={styles.pickerList}>
                                                    {availableSkills.map(skill => {
                                                        const alreadyMounted = mountedSkillIds.has(skill.id);
                                                        return (
                                                            <button
                                                                key={skill.id}
                                                                className={`${styles.pickerItem} ${alreadyMounted ? styles.pickerItemMounted : ''}`}
                                                                disabled={alreadyMounted || mounting}
                                                                onClick={() => handleMountSkill(ch.id, skill.id)}
                                                            >
                                                                <span className={styles.pickerIcon}>
                                                                    {CATEGORY_ICONS[skill.category] || '🧩'}
                                                                </span>
                                                                <span className={styles.pickerName}>{skill.name}</span>
                                                                {alreadyMounted && (
                                                                    <span className={styles.pickerMountedLabel}>已挂载</span>
                                                                )}
                                                                {!alreadyMounted && mounting && (
                                                                    <span className={styles.pickerMountedLabel}>挂载中...</span>
                                                                )}
                                                            </button>
                                                        );
                                                    })}
                                                </div>
                                            )}
                                        </div>
                                    )}

                                    {ch.knowledge_points && ch.knowledge_points.length > 0 && (
                                        <div className={styles.kpList}>
                                            {ch.knowledge_points.map(kp => {
                                                const kpMountedIds = new Set(kp.mounted_skills?.map(s => s.skill_id) || []);

                                                return (
                                                    <div key={kp.id} className={styles.kpItem}>
                                                        <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', width: '100%', gap: '8px' }}>
                                                            <div className={`${styles.kpDot} ${kp.is_key_point ? styles.kpDotKey : ''}`} aria-hidden="true" />
                                                            <span className={styles.kpTitle}>
                                                                {kp.title}
                                                                {kp.is_key_point && ' ⭐'}
                                                            </span>
                                                            <span className={styles.kpDifficulty}>
                                                                难度 {(kp.difficulty * 100).toFixed(0)}%
                                                            </span>
                                                            <button
                                                                className={styles.mountBtn}
                                                                title="挂载技能到此知识点"
                                                                onClick={(e) => openKpSkillPicker(kp.id, e)}
                                                                style={{ marginLeft: 'auto', padding: '0 6px', fontSize: 13, height: 22 }}
                                                            >
                                                                {pickerKpId === kp.id ? '✕' : '+'}
                                                            </button>
                                                        </div>

                                                        {/* KP Skill Picker Dropdown */}
                                                        {pickerKpId === kp.id && (
                                                            <div className={styles.skillPicker} ref={pickerRef} style={{ marginTop: 8, alignSelf: 'flex-start', marginLeft: 24 }}>
                                                                <div className={styles.pickerHeader}>选择要挂载的技能到知识点</div>
                                                                {skillsLoading ? (
                                                                    <div className={styles.pickerLoading}>
                                                                        <div className="spinner" />
                                                                    </div>
                                                                ) : availableSkills.length === 0 ? (
                                                                    <div className={styles.pickerEmpty}>暂无可用技能</div>
                                                                ) : (
                                                                    <div className={styles.pickerList}>
                                                                        {availableSkills.map(skill => {
                                                                            const alreadyMounted = kpMountedIds.has(skill.id);
                                                                            return (
                                                                                <button
                                                                                    key={skill.id}
                                                                                    className={`${styles.pickerItem} ${alreadyMounted ? styles.pickerItemMounted : ''}`}
                                                                                    disabled={alreadyMounted || mounting}
                                                                                    onClick={() => handleMountSkillToKP(kp.id, skill.id)}
                                                                                >
                                                                                    <span className={styles.pickerIcon}>
                                                                                        {CATEGORY_ICONS[skill.category] || '🧩'}
                                                                                    </span>
                                                                                    <span className={styles.pickerName}>{skill.name}</span>
                                                                                    {alreadyMounted && (
                                                                                        <span className={styles.pickerMountedLabel}>已挂载</span>
                                                                                    )}
                                                                                    {!alreadyMounted && mounting && (
                                                                                        <span className={styles.pickerMountedLabel}>挂载中...</span>
                                                                                    )}
                                                                                </button>
                                                                            );
                                                                        })}
                                                                    </div>
                                                                )}
                                                            </div>
                                                        )}

                                                        <div className={styles.kpSkills} style={{ marginLeft: 24, marginTop: 4 }}>
                                                            {kp.mounted_skills?.map(s => (
                                                                <span
                                                                    key={s.id}
                                                                    className={`${styles.skillTag} ${styles.skillTagInteractive}`}
                                                                    title={`点击配置 ${s.skill_id}`}
                                                                    role="button"
                                                                    tabIndex={0}
                                                                    onClick={(e) => openConfigPanel(s, ch.id, kp.id, e)}
                                                                    onKeyDown={handleCardKeyDown}
                                                                >
                                                                    {s.skill_id}
                                                                    <span className={styles.scaffoldBadge}>{s.scaffold_level === 'high' ? '高' : s.scaffold_level === 'medium' ? '中' : '低'}</span>
                                                                </span>
                                                            ))}
                                                        </div>
                                                    </div>
                                                );
                                            })}
                                        </div>
                                    )}
                                </div>
                            );
                        })
                    )}
                </div>

                {/* Right: Upload & Docs */}
                <div className={styles.uploadPanel}>
                    <div className={styles.panelTabs}>
                        <button
                            className={`${styles.panelTabBtn} ${rightTab === 'materials' ? styles.panelTabBtnActive : ''}`}
                            onClick={() => setRightTab('materials')}
                        >
                            📚 教材管理
                        </button>
                        <button
                            className={`${styles.panelTabBtn} ${rightTab === 'activities' ? styles.panelTabBtnActive : ''}`}
                            onClick={() => setRightTab('activities')}
                        >
                            📣 活动发布
                        </button>
                    </div>

                    {rightTab === 'materials' && (
                        <>
                            <div
                                className={`${styles.uploadZone} ${dragging ? styles.uploadZoneDragging : ''}`}
                                role="button"
                                tabIndex={0}
                                aria-label="上传 PDF 文件"
                                onClick={() => fileInput.current?.click()}
                                onKeyDown={handleCardKeyDown}
                                onDragOver={e => { e.preventDefault(); setDragging(true); }}
                                onDragLeave={() => setDragging(false)}
                                onDrop={handleDrop}
                            >
                                <input
                                    ref={fileInput}
                                    type="file"
                                    accept=".pdf"
                                    style={{ display: 'none' }}
                                    onChange={e => {
                                        const file = e.target.files?.[0];
                                        if (file) handleUpload(file);
                                    }}
                                />
                                <div className={styles.uploadIcon}>{uploading ? '⏳' : '📎'}</div>
                                <div className={styles.uploadText}>
                                    {uploading ? '上传中...' : '点击或拖拽 PDF 文件到此处'}
                                </div>
                                <div className={styles.uploadHint}>
                                    上传后 AI 将自动解析并生成知识图谱
                                </div>
                            </div>

                            <div className={styles.docList}>
                                {docs.map(doc => (
                                    <div key={doc.id} className={styles.docItem}>
                                        <span style={{ fontSize: 16 }}>
                                            {doc.status === 'processing' ? '⏳' : doc.status === 'completed' ? '✅' : doc.status === 'failed' ? '❌' : '📄'}
                                        </span>
                                        <span className={styles.docName}>{doc.file_name}</span>
                                        {doc.page_count > 0 && (
                                            <span className={styles.docPages}>{doc.page_count}页</span>
                                        )}
                                        <span className={`badge badge-${doc.status}`}>
                                            {DOCUMENT_STATUS_LABEL[doc.status] || doc.status}
                                        </span>
                                    </div>
                                ))}
                            </div>
                        </>
                    )}

                    {rightTab === 'activities' && (
                        <div className={styles.activityPanel} style={{ marginTop: 0, borderTop: 'none', paddingTop: 0 }}>
                            <button
                                type="button"
                                className="btn btn-primary"
                                style={{ width: '100%' }}
                                disabled={creatingActivity}
                                onClick={handleNewActivity}
                            >
                                {creatingActivity ? '创建中...' : '+ 新建活动'}
                            </button>

                            <div className={styles.activityList}>
                                <div className={styles.activityListTitle}>当前活动</div>
                                {activities.length === 0 ? (
                                    <div className={styles.emptyHint}>暂无活动，点击上方按钮创建</div>
                                ) : (
                                    <div className={styles.activityItems}>
                                        {activities.map((activity) => (
                                            <div
                                                key={activity.id}
                                                className={styles.activityItem}
                                                role="button"
                                                tabIndex={0}
                                                style={{ cursor: 'pointer' }}
                                                onClick={() => router.push(`/teacher/activities/${activity.id}/design`)}
                                                onKeyDown={(e) => {
                                                    if (e.key === 'Enter' || e.key === ' ') {
                                                        e.preventDefault();
                                                        router.push(`/teacher/activities/${activity.id}/design`);
                                                    }
                                                }}
                                            >
                                                <div>
                                                    <div className={styles.activityName}>
                                                        {activity.title}
                                                        <span className={styles.scaffoldBadge} style={{ marginLeft: '8px' }}>
                                                            {activity.type === 'guided' ? '定制化' : '全自主'}
                                                        </span>
                                                    </div>
                                                    <div className={styles.activityMeta}>
                                                        状态：{activity.status}
                                                        {activity.published_at ? ` · 发布于 ${new Date(activity.published_at).toLocaleDateString('zh-CN')}` : ''}
                                                    </div>
                                                </div>
                                                <div className={styles.activityActions}>
                                                    {activity.status !== 'published' && (
                                                        <button
                                                            type="button"
                                                            className="btn btn-secondary"
                                                            disabled={publishingId === activity.id}
                                                            onClick={(e) => {
                                                                e.stopPropagation();
                                                                handlePublishActivity(activity.id);
                                                            }}
                                                        >
                                                            {publishingId === activity.id ? '发布中...' : '发布'}
                                                        </button>
                                                    )}
                                                    {activity.status === 'published' && (
                                                        <span className={styles.scaffoldBadge} style={{ fontSize: 11, padding: '2px 8px' }}>已发布</span>
                                                    )}
                                                </div>
                                            </div>
                                        ))}
                                    </div>
                                )}
                            </div>
                        </div>
                    )}
                </div>
            </div>

            {/* Skill Config Modal */}
            {configMount && (
                <div className={styles.configOverlay} onClick={closeConfigModal}>
                    <div className={styles.configModal} ref={configModalRef} role="dialog" aria-modal="true" aria-labelledby="skill-config-title" tabIndex={-1} onClick={e => e.stopPropagation()}>
                        <div className={styles.configHeader}>
                            <h3 id="skill-config-title" className={styles.configTitle}>技能配置</h3>
                            <span className={styles.configSkillId}>{configMount.mount.skill_id}</span>
                            <button className={styles.configCloseBtn} onClick={closeConfigModal} aria-label="关闭配置面板"><span aria-hidden="true">✕</span></button>
                        </div>

                        <div className={styles.configBody}>
                            {/* Scaffold Level Selector */}
                            <div className={styles.configSection}>
                                <label className={styles.configLabel}>支架强度</label>
                                <p className={styles.configHint}>控制 AI 提供引导的程度：高 = 详细指引，中 = 适度提示，低 = 仅观察</p>
                                <div className={styles.levelSelector}>
                                    {(['high', 'medium', 'low'] as const).map(level => (
                                        <button
                                            key={level}
                                            className={`${styles.levelBtn} ${configLevel === level ? styles.levelBtnActive : ''}`}
                                            onClick={() => setConfigLevel(level)}
                                        >
                                            <span className={styles.levelIcon}>
                                                {level === 'high' ? '🛡️' : level === 'medium' ? '⚖️' : '🔓'}
                                            </span>
                                            <span className={styles.levelName}>
                                                {level === 'high' ? '高' : level === 'medium' ? '中' : '低'}
                                            </span>
                                        </button>
                                    ))}
                                </div>
                            </div>

                            {/* Progressive Rule Editor */}
                            <div className={styles.configSection}>
                                <label className={styles.configLabel}>渐进规则（可选）</label>
                                <p className={styles.configHint}>当学生掌握度达到阈值时，自动降低支架强度</p>

                                <div className={styles.ruleRow}>
                                    <label className={styles.ruleLabel}>掌握度阈值</label>
                                    <input
                                        type="number"
                                        className={styles.ruleInput}
                                        placeholder="例如 0.7"
                                        min="0"
                                        max="1"
                                        step="0.05"
                                        value={configThreshold}
                                        onChange={e => setConfigThreshold(e.target.value)}
                                    />
                                </div>

                                <div className={styles.ruleRow}>
                                    <label className={styles.ruleLabel}>降级到</label>
                                    <select
                                        className={styles.ruleSelect}
                                        value={configDegradeTo}
                                        onChange={e => setConfigDegradeTo(e.target.value)}
                                    >
                                        <option value="">不自动降级</option>
                                        <option value="medium">中 (medium)</option>
                                        <option value="low">低 (low)</option>
                                    </select>
                                </div>
                            </div>
                        </div>

                        <div className={styles.configFooter}>
                            <button
                                className={styles.unmountBtn}
                                onClick={() => handleUnmountSkill(configMount.kpId, configMount.mount.id)}
                                disabled={unmounting === configMount.mount.id}
                            >
                                {unmounting === configMount.mount.id ? '卸载中...' : '卸载技能'}
                            </button>
                            <div className={styles.configActions}>
                                <button className={styles.cancelBtn} onClick={closeConfigModal}>取消</button>
                                <button
                                    className={styles.saveBtn}
                                    onClick={handleSaveConfig}
                                    disabled={configSaving}
                                >
                                    {configSaving ? '保存中...' : '保存配置'}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}

            {/* AI Recommendation Modal */}
            {recommendations && (
                <div className={styles.configOverlay} onClick={() => setRecommendations(null)}>
                    <div className={styles.configModal} style={{ maxWidth: 700 }} onClick={e => e.stopPropagation()}>
                        <div className={styles.configHeader}>
                            <h3 className={styles.configTitle}>🤖 AI 技能挂载建议</h3>
                            <button className={styles.configCloseBtn} onClick={() => setRecommendations(null)} aria-label="关闭建议面板"><span aria-hidden="true">✕</span></button>
                        </div>

                        <div className={styles.configBody} style={{ maxHeight: '60vh', overflowY: 'auto' }}>
                            <p className={styles.configHint} style={{ marginBottom: 16 }}>
                                AI 根据知识点特性和难度，推荐了以下技能挂载方案。您可以取消勾选不需要的挂载。
                            </p>

                            {recommendations.length === 0 ? (
                                <div style={{ textAlign: 'center', padding: 32, color: 'var(--color-text-secondary)' }}>
                                    未找到合适的挂载建议。
                                </div>
                            ) : (
                                <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
                                    {recommendations.map((rec, idx) => (
                                        <div key={idx} style={{
                                            display: 'flex', gap: 12, padding: 12,
                                            border: '1px solid var(--color-border)',
                                            borderRadius: 8,
                                            background: selectedMounts.has(idx.toString()) ? 'var(--color-bg-primary)' : 'var(--color-bg-secondary)',
                                            opacity: selectedMounts.has(idx.toString()) ? 1 : 0.6
                                        }}>
                                            <input
                                                type="checkbox"
                                                checked={selectedMounts.has(idx.toString())}
                                                onChange={() => toggleMountSelection(idx.toString())}
                                                style={{ marginTop: 4 }}
                                            />
                                            <div style={{ flex: 1 }}>
                                                <div style={{ fontWeight: 500, marginBottom: 4 }}>
                                                    {rec.kp_title} <span style={{ color: 'var(--color-text-secondary)', fontWeight: 'normal' }}>— {rec.skill_name}</span>
                                                </div>
                                                <div style={{ fontSize: 13, color: 'var(--color-text-secondary)', marginBottom: 4 }}>
                                                    <strong>推荐理由：</strong>{rec.reason}
                                                </div>
                                                <span className={styles.scaffoldBadge}>
                                                    支架: {rec.scaffold_level === 'high' ? '高' : rec.scaffold_level === 'medium' ? '中' : '低'}
                                                </span>
                                            </div>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>

                        <div className={styles.configFooter}>
                            <div />
                            <div className={styles.configActions}>
                                <button className={styles.cancelBtn} onClick={() => setRecommendations(null)}>取消</button>
                                <button
                                    className={styles.saveBtn}
                                    onClick={handleBatchMount}
                                    disabled={batchMounting || selectedMounts.size === 0}
                                >
                                    {batchMounting ? '挂载中...' : `确认批量挂载 (${selectedMounts.size})`}
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
