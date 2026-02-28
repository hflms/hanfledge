'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { listSkills, getSkillDetail, type SkillMetadata, type SkillConstraints } from '@/lib/api';
import MarkdownRenderer from '@/components/MarkdownRenderer';
import styles from './page.module.css';

// -- Constants ------------------------------------------------

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

/** Teaching stages mapped to recommended skill categories (design §6.1) */
const TEACHING_STAGES: { key: string; label: string; icon: string; categories: string[] }[] = [
    { key: 'all', label: '全部技能', icon: '📦', categories: [] },
    { key: 'concept', label: '概念引入', icon: '💡', categories: ['inquiry-based'] },
    { key: 'practice', label: '练习巩固', icon: '📝', categories: ['critical-thinking'] },
    { key: 'review', label: '复习评估', icon: '🎯', categories: ['role-play', 'collaborative'] },
];

const TOOL_LABELS: Record<string, string> = {
    leveler: '难度调节器',
    make_it_relevant: '时事关联',
};

// -- Component ------------------------------------------------

export default function SkillStorePage() {
    const [skills, setSkills] = useState<SkillMetadata[]>([]);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [subjectFilter, setSubjectFilter] = useState('');
    const [categoryFilter, setCategoryFilter] = useState('');
    const [searchQuery, setSearchQuery] = useState('');
    const [activeStage, setActiveStage] = useState('all');
    const [selectedSkill, setSelectedSkill] = useState<{
        metadata: SkillMetadata;
        constraints: SkillConstraints | null;
    } | null>(null);
    const [detailLoading, setDetailLoading] = useState(false);

    const fetchSkills = useCallback(async () => {
        setError(null);
        try {
            const data = await listSkills(
                subjectFilter || undefined,
                categoryFilter || undefined,
            );
            setSkills(data || []);
        } catch (err) {
            console.error('Failed to fetch skills', err);
            setError('加载技能列表失败，请稍后重试');
        } finally {
            setLoading(false);
        }
    }, [subjectFilter, categoryFilter]);

    useEffect(() => {
        setLoading(true);
        fetchSkills();
    }, [fetchSkills]);

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

    // -- Filtering logic --

    /** Skills filtered by the active teaching stage tab */
    const stageFilteredSkills = useMemo(() => {
        const stage = TEACHING_STAGES.find(s => s.key === activeStage);
        if (!stage || stage.categories.length === 0) return skills;
        return skills.filter(s => stage.categories.includes(s.category));
    }, [skills, activeStage]);

    /** Skills further filtered by text search */
    const filteredSkills = useMemo(() => {
        if (!searchQuery.trim()) return stageFilteredSkills;
        const q = searchQuery.toLowerCase();
        return stageFilteredSkills.filter(s =>
            s.name.toLowerCase().includes(q) ||
            s.description.toLowerCase().includes(q) ||
            s.tags.some(t => t.toLowerCase().includes(q)) ||
            s.id.toLowerCase().includes(q),
        );
    }, [stageFilteredSkills, searchQuery]);

    /** Count of skills per teaching stage (for badge display) */
    const stageCounts = useMemo(() => {
        const counts: Record<string, number> = { all: skills.length };
        for (const stage of TEACHING_STAGES) {
            if (stage.key === 'all') continue;
            counts[stage.key] = skills.filter(s => stage.categories.includes(s.category)).length;
        }
        return counts;
    }, [skills]);

    // -- Render -----------------------------------------------

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
        );
    }

    return (
        <div className="fade-in">
            {/* Page Header */}
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>技能商店</h1>
                <div className={styles.filterBar}>
                    <input
                        className={styles.searchInput}
                        type="text"
                        placeholder="搜索技能名称、描述或标签..."
                        value={searchQuery}
                        onChange={e => setSearchQuery(e.target.value)}
                    />
                    <select
                        className={styles.filterSelect}
                        value={categoryFilter}
                        onChange={e => setCategoryFilter(e.target.value)}
                    >
                        <option value="">全部类型</option>
                        {Object.entries(CATEGORY_MAP).map(([key, label]) => (
                            <option key={key} value={key}>{label}</option>
                        ))}
                    </select>
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

            {/* Teaching Stage Tabs */}
            <div className={styles.stageTabs}>
                {TEACHING_STAGES.map(stage => (
                    <button
                        key={stage.key}
                        className={`${styles.stageTab} ${activeStage === stage.key ? styles.stageTabActive : ''}`}
                        onClick={() => setActiveStage(stage.key)}
                    >
                        <span className={styles.stageTabIcon}>{stage.icon}</span>
                        {stage.label}
                        <span className={styles.stageTabCount}>{stageCounts[stage.key] || 0}</span>
                    </button>
                ))}
            </div>

            {/* Error State */}
            {error && (
                <div className={styles.errorState}>
                    <span>{error}</span>
                    <button className={styles.retryBtn} onClick={() => { setLoading(true); fetchSkills(); }}>
                        重试
                    </button>
                </div>
            )}

            {/* Skill Grid */}
            {filteredSkills.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>🧩</div>
                    <div className={styles.emptyText}>
                        {searchQuery ? `未找到匹配"${searchQuery}"的技能` : '暂无可用技能'}
                    </div>
                    {searchQuery && (
                        <button className={styles.clearSearchBtn} onClick={() => setSearchQuery('')}>
                            清除搜索
                        </button>
                    )}
                </div>
            ) : (
                <div className={styles.skillGrid}>
                    {filteredSkills.map(skill => (
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

                            {/* Tools badges on cards */}
                            {skill.tools && Object.keys(skill.tools).length > 0 && (
                                <div className={styles.toolBadges}>
                                    {Object.entries(skill.tools).map(([key, tool]) => (
                                        tool.enabled && (
                                            <span key={key} className={styles.toolBadge}>
                                                {TOOL_LABELS[key] || key}
                                            </span>
                                        )
                                    ))}
                                </div>
                            )}

                            <div className={styles.skillMeta}>
                                <span>支架: {skill.scaffolding_levels.join(' → ')}</span>
                                <span>作者: {skill.author}</span>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Detail Loading Overlay */}
            {detailLoading && (
                <div className={styles.modalOverlay}>
                    <div className="spinner" />
                </div>
            )}

            {/* Skill Detail Modal */}
            {selectedSkill && !detailLoading && (
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

                            {/* Tools Section */}
                            {selectedSkill.metadata.tools && Object.keys(selectedSkill.metadata.tools).length > 0 && (
                                <div className={styles.detailSection}>
                                    <h4>配置工具</h4>
                                    <div className={styles.toolList}>
                                        {Object.entries(selectedSkill.metadata.tools).map(([key, tool]) => (
                                            <div key={key} className={styles.toolItem}>
                                                <div className={styles.toolHeader}>
                                                    <span className={styles.toolName}>{TOOL_LABELS[key] || key}</span>
                                                    <span className={`${styles.toolStatus} ${tool.enabled ? styles.toolEnabled : styles.toolDisabled}`}>
                                                        {tool.enabled ? '已启用' : '未启用'}
                                                    </span>
                                                </div>
                                                <div className={styles.toolDesc}>{tool.description}</div>
                                            </div>
                                        ))}
                                    </div>
                                </div>
                            )}

                            {/* Progressive Triggers Section */}
                            {selectedSkill.metadata.progressive_triggers && (
                                <div className={styles.detailSection}>
                                    <h4>渐进策略</h4>
                                    <div className={styles.triggerList}>
                                        {selectedSkill.metadata.progressive_triggers.activate_when && (
                                            <div className={styles.triggerItem}>
                                                <span className={styles.triggerLabel}>激活条件</span>
                                                <code className={styles.triggerCode}>
                                                    {selectedSkill.metadata.progressive_triggers.activate_when}
                                                </code>
                                            </div>
                                        )}
                                        {selectedSkill.metadata.progressive_triggers.deactivate_when && (
                                            <div className={styles.triggerItem}>
                                                <span className={styles.triggerLabel}>退出条件</span>
                                                <code className={styles.triggerCode}>
                                                    {selectedSkill.metadata.progressive_triggers.deactivate_when}
                                                </code>
                                            </div>
                                        )}
                                    </div>
                                </div>
                            )}

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

                            <div className={styles.detailSection}>
                                <h4>支架等级</h4>
                                <div className={styles.scaffoldFlow}>
                                    {selectedSkill.metadata.scaffolding_levels.map((level, i) => (
                                        <span key={level} className={styles.scaffoldItem}>
                                            <span className={styles.scaffoldDot} />
                                            {level === 'high' ? '高支架' : level === 'medium' ? '中支架' : level === 'low' ? '低支架' : level}
                                            {i < selectedSkill.metadata.scaffolding_levels.length - 1 && (
                                                <span className={styles.scaffoldArrow}>→</span>
                                            )}
                                        </span>
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
                                    <div className={styles.markdownPreview}>
                                        <MarkdownRenderer content={selectedSkill.constraints.raw_markdown} />
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}
