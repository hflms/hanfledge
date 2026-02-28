'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import {
    getCourseOutline, uploadMaterial, getDocuments,
    listSkills, mountSkill, unmountSkill, updateSkillConfig,
    type Course, type Document, type SkillMetadata, type MountedSkill,
} from '@/lib/api';
import styles from './page.module.css';

const STATUS_LABEL: Record<string, string> = {
    uploaded: '已上传',
    processing: '处理中...',
    completed: '已完成',
    failed: '处理失败',
};

const CATEGORY_ICONS: Record<string, string> = {
    'inquiry-based': '🔍',
    'critical-thinking': '🧐',
    'collaborative': '🤝',
    'role-play': '🎭',
};

export default function OutlinePage() {
    const params = useParams();
    const courseId = Number(params.id);
    const fileInput = useRef<HTMLInputElement>(null);

    const [course, setCourse] = useState<Course | null>(null);
    const [docs, setDocs] = useState<Document[]>([]);
    const [loading, setLoading] = useState(true);
    const [uploading, setUploading] = useState(false);
    const [dragging, setDragging] = useState(false);

    // -- Skill mounting state ----------------------------------------
    const [pickerChapterId, setPickerChapterId] = useState<number | null>(null);
    const [availableSkills, setAvailableSkills] = useState<SkillMetadata[]>([]);
    const [skillsLoading, setSkillsLoading] = useState(false);
    const [mounting, setMounting] = useState(false);
    const [unmounting, setUnmounting] = useState<number | null>(null);
    const pickerRef = useRef<HTMLDivElement>(null);

    // -- Skill config panel state ------------------------------------
    const [configMount, setConfigMount] = useState<{ mount: MountedSkill; chapterId: number } | null>(null);
    const [configLevel, setConfigLevel] = useState<string>('high');
    const [configThreshold, setConfigThreshold] = useState<string>('');
    const [configDegradeTo, setConfigDegradeTo] = useState<string>('');
    const [configSaving, setConfigSaving] = useState(false);

    const fetchData = useCallback(async () => {
        try {
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setDocs(data.documents || []);
        } catch (err) {
            console.error('Failed to fetch outline', err);
        } finally {
            setLoading(false);
        }
    }, [courseId]);

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

    const handleUpload = async (file: File) => {
        if (!file.name.toLowerCase().endsWith('.pdf')) {
            alert('仅支持 PDF 文件');
            return;
        }
        setUploading(true);
        try {
            await uploadMaterial(courseId, file);
            const freshDocs = await getDocuments(courseId);
            setDocs(freshDocs);
        } catch (err) {
            console.error('Upload failed', err);
            alert('上传失败');
        } finally {
            setUploading(false);
        }
    };

    const handleDrop = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setDragging(false);
        const file = e.dataTransfer.files[0];
        if (file) handleUpload(file);
    }, []);

    // -- Skill picker handlers ----------------------------------------

    const openSkillPicker = async (chapterId: number, e: React.MouseEvent) => {
        e.stopPropagation();
        if (pickerChapterId === chapterId) {
            setPickerChapterId(null);
            return;
        }
        setPickerChapterId(chapterId);
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
            alert('挂载技能失败');
        } finally {
            setMounting(false);
        }
    };

    const handleUnmountSkill = async (chapterId: number, mountId: number) => {
        if (!confirm('确认卸载该技能？')) return;
        setUnmounting(mountId);
        try {
            await unmountSkill(chapterId, mountId);
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setConfigMount(null);
        } catch (err) {
            console.error('Failed to unmount skill', err);
            alert('卸载技能失败');
        } finally {
            setUnmounting(null);
        }
    };

    // -- Skill config panel handlers ----------------------------------

    const openConfigPanel = (mount: MountedSkill, chapterId: number, e: React.MouseEvent) => {
        e.stopPropagation();
        setConfigMount({ mount, chapterId });
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

            await updateSkillConfig(configMount.chapterId, configMount.mount.id, {
                scaffold_level: configLevel,
                progressive_rule: Object.keys(progressiveRule).length > 0 ? progressiveRule : undefined,
            });
            const data = await getCourseOutline(courseId);
            setCourse(data.course);
            setConfigMount(null);
        } catch (err) {
            console.error('Failed to save config', err);
            alert('保存配置失败');
        } finally {
            setConfigSaving(false);
        }
    };

    // Close skill picker on outside click
    useEffect(() => {
        if (!pickerChapterId) return;
        const handleClickOutside = (e: MouseEvent) => {
            if (pickerRef.current && !pickerRef.current.contains(e.target as Node)) {
                setPickerChapterId(null);
            }
        };
        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, [pickerChapterId]);

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
        );
    }

    const chapters = course?.chapters || [];

    return (
        <div className="fade-in">
            <Link href="/teacher/courses" className={styles.backLink}>← 返回课程列表</Link>
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>{course?.title || '课程详情'}</h1>
                <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
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
                    <h3 style={{ marginBottom: 16, fontSize: 15, fontWeight: 600 }}>📋 课程大纲</h3>

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
                                            {ch.knowledge_points.map(kp => (
                                                <div key={kp.id} className={styles.kpItem}>
                                                    <div className={`${styles.kpDot} ${kp.is_key_point ? styles.kpDotKey : ''}`} />
                                                    <span className={styles.kpTitle}>
                                                        {kp.title}
                                                        {kp.is_key_point && ' ⭐'}
                                                    </span>
                                                    <span className={styles.kpDifficulty}>
                                                        难度 {(kp.difficulty * 100).toFixed(0)}%
                                                    </span>
                                                    <div className={styles.kpSkills}>
                                                        {kp.mounted_skills?.map(s => (
                                                            <span
                                                                key={s.id}
                                                                className={`${styles.skillTag} ${styles.skillTagInteractive}`}
                                                                title={`点击配置 ${s.skill_id}`}
                                                                onClick={(e) => openConfigPanel(s, ch.id, e)}
                                                            >
                                                                {s.skill_id}
                                                                <span className={styles.scaffoldBadge}>{s.scaffold_level === 'high' ? '高' : s.scaffold_level === 'medium' ? '中' : '低'}</span>
                                                            </span>
                                                        ))}
                                                    </div>
                                                </div>
                                            ))}
                                        </div>
                                    )}
                                </div>
                            );
                        })
                    )}
                </div>

                {/* Right: Upload & Docs */}
                <div className={styles.uploadPanel}>
                    <h3 style={{ marginBottom: 16, fontSize: 15, fontWeight: 600 }}>📤 教材文档</h3>

                    <div
                        className={`${styles.uploadZone} ${dragging ? styles.uploadZoneDragging : ''}`}
                        onClick={() => fileInput.current?.click()}
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
                                    {STATUS_LABEL[doc.status] || doc.status}
                                </span>
                            </div>
                        ))}
                    </div>
                </div>
            </div>

            {/* Skill Config Modal */}
            {configMount && (
                <div className={styles.configOverlay} onClick={() => setConfigMount(null)}>
                    <div className={styles.configModal} onClick={e => e.stopPropagation()}>
                        <div className={styles.configHeader}>
                            <h3 className={styles.configTitle}>技能配置</h3>
                            <span className={styles.configSkillId}>{configMount.mount.skill_id}</span>
                            <button className={styles.configCloseBtn} onClick={() => setConfigMount(null)}>✕</button>
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
                                onClick={() => handleUnmountSkill(configMount.chapterId, configMount.mount.id)}
                                disabled={unmounting === configMount.mount.id}
                            >
                                {unmounting === configMount.mount.id ? '卸载中...' : '卸载技能'}
                            </button>
                            <div className={styles.configActions}>
                                <button className={styles.cancelBtn} onClick={() => setConfigMount(null)}>取消</button>
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
        </div>
    );
}
