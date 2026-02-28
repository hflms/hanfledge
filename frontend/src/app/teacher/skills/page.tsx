'use client';

import { useEffect, useState } from 'react';
import { listSkills, getSkillDetail, type SkillMetadata, type SkillConstraints } from '@/lib/api';
import styles from './page.module.css';

const CATEGORY_MAP: Record<string, string> = {
    'inquiry-based': '探究式教学',
    'critical-thinking': '批判性思维',
    'collaborative': '协作学习',
    'role-play': '角色扮演',
};

const SUBJECT_MAP: Record<string, string> = {
    math: '数学',
    physics: '物理',
    chemistry: '化学',
    biology: '生物',
    chinese: '语文',
    english: '英语',
    history: '历史',
    geography: '地理',
};

const CATEGORY_ICONS: Record<string, string> = {
    'inquiry-based': '🔍',
    'critical-thinking': '🧐',
    'collaborative': '🤝',
    'role-play': '🎭',
};

export default function SkillStorePage() {
    const [skills, setSkills] = useState<SkillMetadata[]>([]);
    const [loading, setLoading] = useState(true);
    const [subjectFilter, setSubjectFilter] = useState('');
    const [selectedSkill, setSelectedSkill] = useState<{
        metadata: SkillMetadata;
        constraints: SkillConstraints | null;
    } | null>(null);
    const [detailLoading, setDetailLoading] = useState(false);

    const fetchSkills = async () => {
        try {
            const data = await listSkills(subjectFilter || undefined);
            setSkills(data || []);
        } catch (err) {
            console.error('Failed to fetch skills', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        setLoading(true);
        fetchSkills();
    }, [subjectFilter]);

    const handleViewDetail = async (skillId: string) => {
        setDetailLoading(true);
        try {
            const detail = await getSkillDetail(skillId);
            setSelectedSkill(detail);
        } catch (err) {
            console.error('Failed to load skill detail', err);
        } finally {
            setDetailLoading(false);
        }
    };

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
        );
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>技能商店</h1>
                <div className={styles.filterBar}>
                    <select
                        className={styles.filterSelect}
                        value={subjectFilter}
                        onChange={e => setSubjectFilter(e.target.value)}
                    >
                        <option value="">全部学科</option>
                        {Object.entries(SUBJECT_MAP).map(([key, label]) => (
                            <option key={key} value={key}>{label}</option>
                        ))}
                    </select>
                </div>
            </div>

            {skills.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>🧩</div>
                    <div className={styles.emptyText}>暂无可用技能</div>
                </div>
            ) : (
                <div className={styles.skillGrid}>
                    {skills.map(skill => (
                        <div
                            key={skill.id}
                            className={`card ${styles.skillCard}`}
                            onClick={() => handleViewDetail(skill.id)}
                        >
                            <div className={styles.skillCardTop}>
                                <div className={styles.skillIcon}>
                                    {CATEGORY_ICONS[skill.category] || '🧩'}
                                </div>
                                <div>
                                    <div className={styles.skillName}>{skill.name}</div>
                                    <div className={styles.skillCategory}>
                                        {CATEGORY_MAP[skill.category] || skill.category}
                                    </div>
                                </div>
                                <span className={styles.skillVersion}>v{skill.version}</span>
                            </div>

                            <div className={styles.skillDesc}>{skill.description}</div>

                            <div className={styles.tagList}>
                                {skill.subjects.map(s => (
                                    <span key={s} className={styles.subjectTag}>
                                        {SUBJECT_MAP[s] || s}
                                    </span>
                                ))}
                            </div>

                            <div className={styles.tagList}>
                                {skill.tags.map(tag => (
                                    <span key={tag} className={styles.skillTag}>{tag}</span>
                                ))}
                            </div>

                            <div className={styles.skillMeta}>
                                <span>支架: {skill.scaffolding_levels.join(' → ')}</span>
                                <span>作者: {skill.author}</span>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Skill Detail Modal */}
            {selectedSkill && (
                <div className={styles.modalOverlay} onClick={() => setSelectedSkill(null)}>
                    <div className={styles.modal} onClick={e => e.stopPropagation()}>
                        <div className={styles.modalHeader}>
                            <div className={styles.modalIcon}>
                                {CATEGORY_ICONS[selectedSkill.metadata.category] || '🧩'}
                            </div>
                            <div>
                                <h2 className={styles.modalTitle}>{selectedSkill.metadata.name}</h2>
                                <div className={styles.modalSubtitle}>
                                    {CATEGORY_MAP[selectedSkill.metadata.category] || selectedSkill.metadata.category}
                                    {' · '}v{selectedSkill.metadata.version}
                                    {' · '}{selectedSkill.metadata.author}
                                </div>
                            </div>
                            <button className={styles.closeBtn} onClick={() => setSelectedSkill(null)}>✕</button>
                        </div>

                        <div className={styles.modalBody}>
                            <div className={styles.detailSection}>
                                <h4>描述</h4>
                                <p>{selectedSkill.metadata.description}</p>
                            </div>

                            <div className={styles.detailSection}>
                                <h4>适用学科</h4>
                                <div className={styles.tagList}>
                                    {selectedSkill.metadata.subjects.map(s => (
                                        <span key={s} className={styles.subjectTag}>{SUBJECT_MAP[s] || s}</span>
                                    ))}
                                </div>
                            </div>

                            <div className={styles.detailSection}>
                                <h4>约束规则</h4>
                                <div className={styles.constraintList}>
                                    {Object.entries(selectedSkill.metadata.constraints).map(([key, val]) => (
                                        <div key={key} className={styles.constraintItem}>
                                            <span className={styles.constraintKey}>{key}</span>
                                            <span className={styles.constraintVal}>{String(val)}</span>
                                        </div>
                                    ))}
                                </div>
                            </div>

                            {selectedSkill.metadata.evaluation_dimensions && (
                                <div className={styles.detailSection}>
                                    <h4>评估维度</h4>
                                    <div className={styles.tagList}>
                                        {selectedSkill.metadata.evaluation_dimensions.map(d => (
                                            <span key={d} className={styles.evalTag}>{d}</span>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {selectedSkill.constraints?.raw_markdown && (
                                <div className={styles.detailSection}>
                                    <h4>SKILL.md 指令</h4>
                                    <pre className={styles.markdownPreview}>
                                        {selectedSkill.constraints.raw_markdown}
                                    </pre>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
