'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import {
    getInquiryTree,
    getInteractionLog,
    type InquiryTreeResponse,
    type InquiryTreeNode,
    type InteractionLogResponse,
    type InteractionLogEntry,
} from '@/lib/api';
import styles from './page.module.css';

// -- Tab Types ------------------------------------------------

type TabType = 'tree' | 'log';

const TAB_LABELS: Record<TabType, string> = {
    tree: '追问深度树',
    log: 'AI 交互日志',
};

// -- Eval Score Helpers ---------------------------------------

const SCORE_COLORS = {
    high: 'var(--success)',
    mid: 'var(--warning)',
    low: 'var(--danger)',
};

function scoreColor(score: number | undefined): string {
    if (score === undefined || score === null) return 'var(--text-muted)';
    if (score >= 0.7) return SCORE_COLORS.high;
    if (score >= 0.4) return SCORE_COLORS.mid;
    return SCORE_COLORS.low;
}

function formatScore(score: number | undefined): string {
    if (score === undefined || score === null) return '-';
    return (score * 100).toFixed(0) + '%';
}

// -- Turn Type Labels -----------------------------------------

const TURN_TYPE_MAP: Record<string, { label: string; color: string }> = {
    question: { label: '提问', color: 'var(--accent)' },
    probe: { label: '追问', color: '#e17055' },
    correction: { label: '纠正', color: '#fdcb6e' },
    response: { label: '回应', color: '#00b894' },
    scaffold_change: { label: '支架变更', color: 'var(--text-muted)' },
};

const ROLE_LABELS: Record<string, string> = {
    student: '学生',
    coach: 'AI 教练',
    system: '系统',
};

const SCAFFOLD_MAP: Record<string, string> = {
    high: '高',
    medium: '中',
    low: '低',
};

// -- Main Component -------------------------------------------

export default function SessionAnalyticsPage() {
    const params = useParams();
    const router = useRouter();
    const sessionId = Number(params.id);

    const [activeTab, setActiveTab] = useState<TabType>('tree');
    const [treeData, setTreeData] = useState<InquiryTreeResponse | null>(null);
    const [logData, setLogData] = useState<InteractionLogResponse | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);

    const loadData = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const [tree, log] = await Promise.all([
                getInquiryTree(sessionId),
                getInteractionLog(sessionId),
            ]);
            setTreeData(tree);
            setLogData(log);
        } catch (err) {
            setError(err instanceof Error ? err.message : '加载失败');
        } finally {
            setLoading(false);
        }
    }, [sessionId]);

    useEffect(() => {
        if (sessionId) loadData();
    }, [sessionId, loadData]);

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
                <div className="spinner" />
            </div>
        );
    }

    if (error) {
        return (
            <div className="fade-in">
                <div className={styles.pageHeader}>
                    <button className="btn btn-ghost" onClick={() => router.back()}>
                        &larr; 返回
                    </button>
                </div>
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>!</div>
                    <div className={styles.emptyText}>{error}</div>
                </div>
            </div>
        );
    }

    return (
        <div className="fade-in">
            {/* Header */}
            <div className={styles.pageHeader}>
                <div className={styles.headerLeft}>
                    <button className="btn btn-ghost btn-sm" onClick={() => router.back()}>
                        &larr; 返回
                    </button>
                    <h1 className={styles.pageTitle}>
                        会话分析 #{sessionId}
                    </h1>
                    {treeData && (
                        <span className={styles.studentName}>{treeData.student_name}</span>
                    )}
                </div>
            </div>

            {/* Summary Stats */}
            {treeData && logData && (
                <div className={styles.statsRow}>
                    <div className={styles.statCard}>
                        <div className={styles.statLabel}>总对话轮数</div>
                        <div className={styles.statValue}>{treeData.total_turns}</div>
                    </div>
                    <div className={styles.statCard}>
                        <div className={styles.statLabel}>最大追问深度</div>
                        <div className={styles.statValue}>{treeData.max_depth}</div>
                    </div>
                    <div className={styles.statCard}>
                        <div className={styles.statLabel}>教学技能</div>
                        <div className={styles.statValue} style={{ fontSize: 16 }}>
                            {treeData.skill_used || '-'}
                        </div>
                    </div>
                    <div className={styles.statCard}>
                        <div className={styles.statLabel}>支架等级</div>
                        <div className={styles.statValue}>
                            {SCAFFOLD_MAP[logData.scaffold_level] || logData.scaffold_level}
                        </div>
                    </div>
                </div>
            )}

            {/* Tab Navigation */}
            <div className={styles.tabBar}>
                {(Object.keys(TAB_LABELS) as TabType[]).map(tab => (
                    <button
                        key={tab}
                        className={`${styles.tab} ${activeTab === tab ? styles.tabActive : ''}`}
                        onClick={() => setActiveTab(tab)}
                    >
                        {TAB_LABELS[tab]}
                    </button>
                ))}
            </div>

            {/* Tab Content */}
            <div className={styles.tabContent}>
                {activeTab === 'tree' && treeData && (
                    <InquiryTree data={treeData} />
                )}
                {activeTab === 'log' && logData && (
                    <InteractionLog data={logData} />
                )}
            </div>
        </div>
    );
}

// -- Inquiry Depth Tree Component -----------------------------

function InquiryTree({ data }: { data: InquiryTreeResponse }) {
    if (!data.roots || data.roots.length === 0) {
        return (
            <div className={styles.emptyState}>
                <div className={styles.emptyIcon}>🌳</div>
                <div className={styles.emptyText}>暂无追问数据</div>
            </div>
        );
    }

    return (
        <div className={styles.treeContainer}>
            {data.roots.map((root, i) => (
                <TreeNode key={root.id || i} node={root} isLast={i === data.roots.length - 1} />
            ))}
        </div>
    );
}

function TreeNode({ node, isLast }: { node: InquiryTreeNode; isLast: boolean }) {
    const [expanded, setExpanded] = useState(true);
    const hasChildren = node.children && node.children.length > 0;
    const turnInfo = TURN_TYPE_MAP[node.turn_type] || { label: node.turn_type, color: 'var(--text-muted)' };

    return (
        <div className={styles.treeNode}>
            <div className={styles.treeNodeLine}>
                {/* Vertical connector */}
                <div className={styles.treeConnector}>
                    <div className={`${styles.treeDot} ${node.role === 'student' ? styles.treeDotStudent : styles.treeDotCoach}`} />
                    {!isLast && <div className={styles.treeLine} />}
                </div>

                {/* Content */}
                <div
                    className={styles.treeNodeContent}
                    onClick={() => hasChildren && setExpanded(!expanded)}
                    style={{ cursor: hasChildren ? 'pointer' : 'default' }}
                >
                    <div className={styles.treeNodeHeader}>
                        <span className={styles.treeRole}>
                            {ROLE_LABELS[node.role] || node.role}
                        </span>
                        <span
                            className={styles.treeTurnType}
                            style={{ color: turnInfo.color, borderColor: turnInfo.color }}
                        >
                            {turnInfo.label}
                        </span>
                        <span className={styles.treeDepthBadge}>
                            D{node.depth}
                        </span>
                        <span className={styles.treeTime}>
                            {new Date(node.time).toLocaleTimeString('zh-CN')}
                        </span>
                        {hasChildren && (
                            <span className={styles.treeExpand}>
                                {expanded ? '▼' : '▶'} {node.children!.length}
                            </span>
                        )}
                    </div>
                    <div className={styles.treeNodeText}>
                        {node.content.length > 200
                            ? node.content.slice(0, 200) + '...'
                            : node.content}
                    </div>
                </div>
            </div>

            {/* Children */}
            {expanded && hasChildren && (
                <div className={styles.treeChildren}>
                    {node.children!.map((child, i) => (
                        <TreeNode
                            key={child.id || i}
                            node={child}
                            isLast={i === node.children!.length - 1}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}

// -- Interaction Log Replay Component -------------------------

function InteractionLog({ data }: { data: InteractionLogResponse }) {
    const [selectedEntry, setSelectedEntry] = useState<InteractionLogEntry | null>(null);

    if (data.interactions.length === 0) {
        return (
            <div className={styles.emptyState}>
                <div className={styles.emptyIcon}>💬</div>
                <div className={styles.emptyText}>暂无交互记录</div>
            </div>
        );
    }

    return (
        <div className={styles.logContainer}>
            {/* Timeline */}
            <div className={styles.logTimeline}>
                {data.interactions.map((entry) => (
                    <div
                        key={entry.id}
                        className={`${styles.logEntry} ${entry.role === 'student' ? styles.logEntryStudent : styles.logEntryCoach}`}
                        onClick={() => setSelectedEntry(entry)}
                    >
                        <div className={styles.logEntryHeader}>
                            <span className={styles.logRole}>
                                {ROLE_LABELS[entry.role] || entry.role}
                            </span>
                            <span className={styles.logTime}>
                                {new Date(entry.created_at).toLocaleTimeString('zh-CN')}
                            </span>
                            {entry.role === 'coach' && entry.eval_status === 'evaluated' && (
                                <div className={styles.logScores}>
                                    <span
                                        className={styles.logScorePill}
                                        style={{ color: scoreColor(entry.faithfulness_score) }}
                                        title="忠实度"
                                    >
                                        F {formatScore(entry.faithfulness_score)}
                                    </span>
                                    <span
                                        className={styles.logScorePill}
                                        style={{ color: scoreColor(entry.actionability_score) }}
                                        title="可执行性"
                                    >
                                        A {formatScore(entry.actionability_score)}
                                    </span>
                                    <span
                                        className={styles.logScorePill}
                                        style={{ color: scoreColor(entry.answer_restraint_score) }}
                                        title="答案克制度"
                                    >
                                        R {formatScore(entry.answer_restraint_score)}
                                    </span>
                                </div>
                            )}
                            {entry.role === 'coach' && entry.eval_status === 'pending' && (
                                <span className={styles.logEvalPending}>评估中...</span>
                            )}
                        </div>
                        <div className={styles.logContent}>
                            {entry.content.length > 300
                                ? entry.content.slice(0, 300) + '...'
                                : entry.content}
                        </div>
                        {entry.tokens_used > 0 && (
                            <div className={styles.logMeta}>
                                {entry.tokens_used} tokens
                                {entry.skill_id && ` | ${entry.skill_id}`}
                            </div>
                        )}
                    </div>
                ))}
            </div>

            {/* Detail Modal */}
            {selectedEntry && (
                <div className={styles.modalOverlay} onClick={() => setSelectedEntry(null)}>
                    <div className={styles.modal} onClick={e => e.stopPropagation()}>
                        <div className={styles.modalTitle}>
                            {ROLE_LABELS[selectedEntry.role] || selectedEntry.role} — 完整内容
                        </div>
                        <div className={styles.logDetailContent}>
                            {selectedEntry.content}
                        </div>
                        {selectedEntry.role === 'coach' && selectedEntry.eval_status === 'evaluated' && (
                            <div className={styles.logDetailScores}>
                                <h3 className={styles.logDetailScoreTitle}>RAGAS 评估分数</h3>
                                <div className={styles.scoreGrid}>
                                    <ScoreBar label="忠实度" value={selectedEntry.faithfulness_score} />
                                    <ScoreBar label="可执行性" value={selectedEntry.actionability_score} />
                                    <ScoreBar label="答案克制" value={selectedEntry.answer_restraint_score} />
                                </div>
                            </div>
                        )}
                        <div className={styles.modalClose}>
                            <button className="btn btn-secondary" onClick={() => setSelectedEntry(null)}>
                                关闭
                            </button>
                        </div>
                    </div>
                </div>
            )}
        </div>
    );
}

// -- Score Bar Component --------------------------------------

function ScoreBar({ label, value }: { label: string; value?: number }) {
    const pct = value !== undefined && value !== null ? Math.round(value * 100) : 0;
    const color = scoreColor(value);

    return (
        <div className={styles.scoreBarItem}>
            <div className={styles.scoreBarLabel}>
                <span>{label}</span>
                <span style={{ color }}>{formatScore(value)}</span>
            </div>
            <div className={styles.scoreBarTrack}>
                <div
                    className={styles.scoreBarFill}
                    style={{ width: `${pct}%`, background: color }}
                />
            </div>
        </div>
    );
}
