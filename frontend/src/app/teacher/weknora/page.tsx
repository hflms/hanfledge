'use client';

import { useEffect, useState } from 'react';
import {
    listWeKnoraKnowledgeBases,
    type WeKnoraKB,
} from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

export default function WeKnoraPage() {
    const [loading, setLoading] = useState(true);
    const [knowledgeBases, setKnowledgeBases] = useState<WeKnoraKB[]>([]);
    const [loadError, setLoadError] = useState<string | null>(null);

    useEffect(() => {
        const fetchData = async () => {
            try {
                const data = await listWeKnoraKnowledgeBases();
                setKnowledgeBases(data || []);
                setLoadError(null);
            } catch (err) {
                setKnowledgeBases([]);
                setLoadError('WeKnora 服务未启用或不可用，请确认后端配置与服务状态。');
                console.warn('Failed to fetch WeKnora knowledge bases', err);
            } finally {
                setLoading(false);
            }
        };
        fetchData();
    }, []);

    if (loading) {
        return <LoadingSpinner />;
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <div>
                    <h1 className={styles.pageTitle}>WeKnora 知识库</h1>
                    <p className={styles.pageSubtitle}>
                        查看可用的 WeKnora 知识库，并在课程中进行绑定使用。
                    </p>
                </div>
            </div>

            {knowledgeBases.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>📚</div>
                    <div className={styles.emptyText}>暂无可用的 WeKnora 知识库</div>
                </div>
            ) : (
                <div className={styles.kbGrid}>
                    {knowledgeBases.map((kb) => (
                        <div key={kb.id} className={styles.kbCard}>
                            <div className={styles.kbCardHeader}>
                                <span className={styles.kbIcon}>🧠</span>
                                <div className={styles.kbInfo}>
                                    <div className={styles.kbName}>{kb.name}</div>
                                    <div className={styles.kbMeta}>
                                        {kb.file_count} 个文件 · {kb.chunk_count} 个知识块
                                    </div>
                                </div>
                            </div>
                            {kb.description && (
                                <div className={styles.kbDesc}>{kb.description}</div>
                            )}
                        </div>
                    ))}
                </div>
            )}

            <div className={styles.noticeCard}>
                {loadError && (
                    <div className={styles.noticeError}>{loadError}</div>
                )}
                <div className={styles.noticeTitle}>如何在课程中使用</div>
                <ol className={styles.noticeList}>
                    <li>进入「课程管理」并打开对应课程。</li>
                    <li>在课程页面的「教材管理」中绑定 WeKnora 知识库。</li>
                    <li>绑定完成后，AI 助手即可检索知识库内容。</li>
                </ol>
            </div>
        </div>
    );
}
