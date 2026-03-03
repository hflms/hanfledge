'use client';

import { useEffect, useState, useCallback, useRef } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import {
    getDocuments, uploadMaterial, deleteDocument, retryDocument,
    listWeKnoraKnowledgeBases, getCourseWeKnoraRefs, bindWeKnoraKnowledgeBase, unbindWeKnoraKnowledgeBase,
    type Document, type WeKnoraKB, type WeKnoraKBRef
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import { DOCUMENT_STATUS_LABEL, DOCUMENT_STATUS_ICON } from '@/lib/constants';
import { handleCardKeyDown } from '@/lib/a11y';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

const MAX_FILE_SIZE = 50 * 1024 * 1024; // 50 MB

// -- Upload Queue Types --------------------------------------

interface UploadTask {
    id: string;
    file: File;
    status: 'queued' | 'uploading' | 'done' | 'error';
    progress: number; // 0-100
    error?: string;
}

// -- Helper: format file size --------------------------------

function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// -- Helper: format date -------------------------------------

function formatDate(dateStr: string): string {
    const d = new Date(dateStr);
    const month = String(d.getMonth() + 1).padStart(2, '0');
    const day = String(d.getDate()).padStart(2, '0');
    const hour = String(d.getHours()).padStart(2, '0');
    const min = String(d.getMinutes()).padStart(2, '0');
    return `${month}-${day} ${hour}:${min}`;
}

// -- Component -----------------------------------------------

export default function MaterialsPage() {
    const params = useParams();
    const courseId = Number(params.id);
    const { toast } = useToast();
    const fileInput = useRef<HTMLInputElement>(null);

    const [docs, setDocs] = useState<Document[]>([]);
    const [loading, setLoading] = useState(true);
    const [dragging, setDragging] = useState(false);
    const [uploadQueue, setUploadQueue] = useState<UploadTask[]>([]);
    const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null);
    const [retrying, setRetrying] = useState<number | null>(null);
    const processingRef = useRef(false);

    // -- WeKnora State --
    const [weknoraKBs, setWeknoraKBs] = useState<WeKnoraKB[]>([]);
    const [courseRefs, setCourseRefs] = useState<WeKnoraKBRef[]>([]);
    const [loadingWeKnora, setLoadingWeKnora] = useState(false);
    const [bindingKbId, setBindingKbId] = useState<string | null>(null);
    const [unbindingRefId, setUnbindingRefId] = useState<number | null>(null);

    // -- Fetch documents -----------------------------------------

    const fetchDocs = useCallback(async () => {
        try {
            const data = await getDocuments(courseId);
            setDocs(data);
        } catch (err) {
            console.error('Failed to fetch documents', err);
        } finally {
            setLoading(false);
        }
    }, [courseId]);

    const fetchWeKnora = useCallback(async () => {
        setLoadingWeKnora(true);
        try {
            const [kbs, refs] = await Promise.all([
                listWeKnoraKnowledgeBases().catch(() => [] as WeKnoraKB[]),
                getCourseWeKnoraRefs(courseId).catch(() => [] as WeKnoraKBRef[])
            ]);
            setWeknoraKBs(kbs);
            setCourseRefs(refs);
        } catch (err) {
            console.warn('Failed to fetch WeKnora data', err);
        } finally {
            setLoadingWeKnora(false);
        }
    }, [courseId]);

    useEffect(() => {
        fetchDocs();
        fetchWeKnora();
    }, [fetchDocs, fetchWeKnora]);

    // Poll while any doc is processing
    useEffect(() => {
        const hasProcessing = docs.some(d => d.status === 'processing' || d.status === 'uploaded');
        if (!hasProcessing) return;

        const interval = setInterval(async () => {
            const freshDocs = await getDocuments(courseId);
            setDocs(freshDocs);
        }, 3000);

        return () => clearInterval(interval);
    }, [docs, courseId]);

    // -- Upload queue processing ---------------------------------

    const processQueue = useCallback(async (queue: UploadTask[]) => {
        if (processingRef.current) return;
        processingRef.current = true;

        const pending = queue.filter(t => t.status === 'queued');
        for (const task of pending) {
            // Mark as uploading
            setUploadQueue(prev =>
                prev.map(t => t.id === task.id ? { ...t, status: 'uploading' as const, progress: 10 } : t)
            );

            try {
                // Simulate progress stages (we can't get real XHR progress with apiFetch)
                setUploadQueue(prev =>
                    prev.map(t => t.id === task.id ? { ...t, progress: 30 } : t)
                );

                await uploadMaterial(courseId, task.file);

                setUploadQueue(prev =>
                    prev.map(t => t.id === task.id ? { ...t, status: 'done' as const, progress: 100 } : t)
                );
            } catch (err) {
                const errorMsg = err instanceof Error ? err.message : '上传失败';
                setUploadQueue(prev =>
                    prev.map(t => t.id === task.id ? { ...t, status: 'error' as const, error: errorMsg } : t)
                );
            }
        }

        processingRef.current = false;

        // Refresh document list after batch completes
        const freshDocs = await getDocuments(courseId);
        setDocs(freshDocs);
    }, [courseId]);

    // -- Add files to queue --------------------------------------

    const enqueueFiles = useCallback((files: FileList | File[]) => {
        const newTasks: UploadTask[] = [];
        const errors: string[] = [];

        for (const file of Array.from(files)) {
            if (!file.name.toLowerCase().endsWith('.pdf')) {
                errors.push(`${file.name}: 仅支持 PDF 格式`);
                continue;
            }
            if (file.size > MAX_FILE_SIZE) {
                errors.push(`${file.name}: 文件大小超过 50 MB`);
                continue;
            }

            newTasks.push({
                id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
                file,
                status: 'queued',
                progress: 0,
            });
        }

        if (errors.length > 0) {
            toast(errors.join('\n'), 'warning');
        }

        if (newTasks.length > 0) {
            setUploadQueue(prev => {
                const updated = [...prev, ...newTasks];
                // Start processing after state update
                setTimeout(() => processQueue(updated), 0);
                return updated;
            });
        }
    }, [processQueue, toast]);

    // -- Drag & drop handlers ------------------------------------

    const handleDrop = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setDragging(false);
        if (e.dataTransfer.files.length > 0) {
            enqueueFiles(e.dataTransfer.files);
        }
    }, [enqueueFiles]);

    const handleDragOver = useCallback((e: React.DragEvent) => {
        e.preventDefault();
        setDragging(true);
    }, []);

    const handleDragLeave = useCallback(() => {
        setDragging(false);
    }, []);

    // -- File input handler --------------------------------------

    const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        const files = e.target.files;
        if (files && files.length > 0) {
            enqueueFiles(files);
        }
        // Reset input so the same file can be re-selected
        e.target.value = '';
    }, [enqueueFiles]);

    // -- Delete handler ------------------------------------------

    const handleDelete = async (docId: number) => {
        try {
            await deleteDocument(courseId, docId);
            setDocs(prev => prev.filter(d => d.id !== docId));
            setDeleteConfirm(null);
        } catch (err) {
            const msg = err instanceof Error ? err.message : '删除失败';
            toast(msg, 'error');
        }
    };

    // -- Retry handler -------------------------------------------

    const handleRetry = async (docId: number) => {
        setRetrying(docId);
        try {
            await retryDocument(courseId, docId);
            const freshDocs = await getDocuments(courseId);
            setDocs(freshDocs);
        } catch (err) {
            const msg = err instanceof Error ? err.message : '重试失败';
            toast(msg, 'error');
        } finally {
            setRetrying(null);
        }
    };

    // -- Clear completed uploads ---------------------------------

    const clearCompletedUploads = () => {
        setUploadQueue(prev => prev.filter(t => t.status !== 'done' && t.status !== 'error'));
    };

    // -- WeKnora Handlers --
    const handleBindKb = async (kbId: string) => {
        setBindingKbId(kbId);
        try {
            await bindWeKnoraKnowledgeBase(courseId, kbId);
            toast('知识库绑定成功', 'success');
            await fetchWeKnora();
        } catch {
            toast('绑定失败', 'error');
        } finally {
            setBindingKbId(null);
        }
    };

    const handleUnbindKb = async (refId: number) => {
        if (!confirm('确定要解绑该知识库吗？')) return;
        setUnbindingRefId(refId);
        try {
            await unbindWeKnoraKnowledgeBase(courseId, refId);
            toast('已解绑', 'success');
            await fetchWeKnora();
        } catch {
            toast('解绑失败', 'error');
        } finally {
            setUnbindingRefId(null);
        }
    };


    // -- Render --------------------------------------------------

    const activeUploads = uploadQueue.filter(t => t.status === 'queued' || t.status === 'uploading');
    const hasFinished = uploadQueue.some(t => t.status === 'done' || t.status === 'error');
    const totalDocs = docs.length;
    const completedDocs = docs.filter(d => d.status === 'completed').length;
    const failedDocs = docs.filter(d => d.status === 'failed').length;

    if (loading) {
        return (
            <LoadingSpinner />
        );
    }

    return (
        <div className="fade-in">
            <Link href={`/teacher/courses/${courseId}/outline`} className={styles.backLink}>
                ← 返回课程大纲
            </Link>

            <div className={styles.pageHeader}>
                <div>
                    <h1 className={styles.pageTitle}>教材管理</h1>
                    <p className={styles.pageSubtitle}>
                        上传 PDF 教材文件，AI 将自动解析并生成知识图谱
                    </p>
                </div>
                <div className={styles.statsRow}>
                    <div className={styles.statBadge}>
                        <span className={styles.statNum}>{totalDocs}</span>
                        <span className={styles.statLabel}>总文档</span>
                    </div>
                    <div className={styles.statBadge}>
                        <span className={styles.statNum}>{completedDocs}</span>
                        <span className={styles.statLabel}>已完成</span>
                    </div>
                    {failedDocs > 0 && (
                        <div className={`${styles.statBadge} ${styles.statBadgeDanger}`}>
                            <span className={styles.statNum}>{failedDocs}</span>
                            <span className={styles.statLabel}>失败</span>
                        </div>
                    )}
                </div>
            </div>

            {/* Upload Zone */}
            <div
                className={`${styles.uploadZone} ${dragging ? styles.uploadZoneDragging : ''}`}
                role="button"
                tabIndex={0}
                aria-label="上传 PDF 文件"
                onClick={() => fileInput.current?.click()}
                onKeyDown={handleCardKeyDown}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                onDrop={handleDrop}
            >
                <input
                    ref={fileInput}
                    type="file"
                    accept=".pdf"
                    multiple
                    style={{ display: 'none' }}
                    onChange={handleFileSelect}
                />
                <div className={styles.uploadIcon}>
                    {activeUploads.length > 0 ? '⏳' : '📎'}
                </div>
                <div className={styles.uploadTitle}>
                    {activeUploads.length > 0
                        ? `正在上传 ${activeUploads.length} 个文件...`
                        : '点击或拖拽 PDF 文件到此处'}
                </div>
                <div className={styles.uploadHint}>
                    支持批量上传 | 仅限 PDF 格式 | 单文件最大 50 MB
                </div>
            </div>

            {/* Upload Queue */}
            {uploadQueue.length > 0 && (
                <div className={styles.queueSection}>
                    <div className={styles.queueHeader}>
                        <span className={styles.queueTitle}>上传队列</span>
                        {hasFinished && (
                            <button className={styles.clearBtn} onClick={clearCompletedUploads}>
                                清除已完成
                            </button>
                        )}
                    </div>
                    <div className={styles.queueList}>
                        {uploadQueue.map(task => (
                            <div key={task.id} className={styles.queueItem}>
                                <span className={styles.queueIcon}>
                                    {task.status === 'done' ? '✅' :
                                     task.status === 'error' ? '❌' :
                                     task.status === 'uploading' ? '⏳' : '📄'}
                                </span>
                                <span className={styles.queueName}>{task.file.name}</span>
                                <span className={styles.queueSize}>{formatSize(task.file.size)}</span>
                                {(task.status === 'uploading' || task.status === 'queued') && (
                                    <div className={styles.progressBar}>
                                        <div
                                            className={styles.progressFill}
                                            style={{ width: `${task.progress}%` }}
                                        />
                                    </div>
                                )}
                                {task.status === 'error' && (
                                    <span className={styles.queueError}>{task.error}</span>
                                )}
                                {task.status === 'done' && (
                                    <span className={styles.queueDone}>已上传</span>
                                )}
                            </div>
                        ))}
                    </div>
                </div>
            )}

            {/* Document List */}
            <div className={styles.docSection}>
                <h3 className={styles.docSectionTitle}>已上传文档</h3>

                {docs.length === 0 ? (
                    <div className={styles.emptyState}>
                        <div style={{ fontSize: 40, marginBottom: 12 }}>📂</div>
                        <p>暂无文档，请上传 PDF 教材开始使用</p>
                    </div>
                ) : (
                    <div className={styles.docGrid}>
                        {docs.map(doc => (
                            <div key={doc.id} className={styles.docCard}>
                                <div className={styles.docCardHeader}>
                                    <span className={styles.docIcon}>
                                        {DOCUMENT_STATUS_ICON[doc.status] || '📄'}
                                    </span>
                                    <div className={styles.docInfo}>
                                        <span className={styles.docName}>{doc.file_name}</span>
                                        <span className={styles.docMeta}>
                                            {doc.page_count > 0 && `${doc.page_count} 页 · `}
                                            {formatDate(doc.created_at)}
                                        </span>
                                    </div>
                                    <span className={`${styles.statusBadge} ${styles[`status_${doc.status}`]}`}>
                                        {DOCUMENT_STATUS_LABEL[doc.status] || doc.status}
                                    </span>
                                </div>

                                {/* Processing progress indicator */}
                                {doc.status === 'processing' && (
                                    <div className={styles.processingBar}>
                                        <div className={styles.processingFill} />
                                    </div>
                                )}

                                
                                {/* Actions & Errors */}
                                {doc.status === 'failed' && doc.error_message && (
                                    <div className={styles.docErrorMsg}>
                                        ⚠️ {doc.error_message}
                                    </div>
                                )}
                                <div className={styles.docActions}>
                                    {doc.status === 'failed' && (
                                        <button
                                            className={styles.retryBtn}
                                            onClick={() => handleRetry(doc.id)}
                                            disabled={retrying === doc.id}
                                        >
                                            {retrying === doc.id ? '重试中...' : '🔄 重试'}
                                        </button>
                                    )}

                                    {doc.status !== 'processing' && (
                                        <>
                                            {deleteConfirm === doc.id ? (
                                                <div className={styles.confirmGroup}>
                                                    <span className={styles.confirmText}>确认删除?</span>
                                                    <button
                                                        className={styles.confirmYes}
                                                        onClick={() => handleDelete(doc.id)}
                                                    >
                                                        是
                                                    </button>
                                                    <button
                                                        className={styles.confirmNo}
                                                        onClick={() => setDeleteConfirm(null)}
                                                    >
                                                        否
                                                    </button>
                                                </div>
                                            ) : (
                                                <button
                                                    className={styles.deleteBtn}
                                                    onClick={() => setDeleteConfirm(doc.id)}
                                                >
                                                    删除
                                                </button>
                                            )}
                                        </>
                                    )}
                                </div>
                            </div>
                        ))}
                    </div>
                )}
            </div>

            {/* WeKnora Integration Section */}
            {(!loadingWeKnora && (weknoraKBs.length > 0 || courseRefs.length > 0)) && (
                <div className={styles.docSection} style={{ marginTop: '32px' }}>
                    <h3 className={styles.docSectionTitle}>WeKnora 外部知识库</h3>
                    <p style={{ color: 'var(--color-text-secondary)', marginBottom: '16px', fontSize: '14px' }}>
                        绑定 WeKnora 知识库后，课程内的 AI 助手将能检索其中的内容来回答学生问题。
                    </p>

                    <div className={styles.docGrid}>
                        {weknoraKBs.map(kb => {
                            const ref = courseRefs.find(r => r.kb_id === kb.id);
                            const isBound = !!ref;

                            return (
                                <div key={kb.id} className={styles.docCard} style={{ borderColor: isBound ? 'var(--color-primary)' : undefined }}>
                                    <div className={styles.docCardHeader}>
                                        <span className={styles.docIcon}>📚</span>
                                        <div className={styles.docInfo}>
                                            <span className={styles.docName}>{kb.name}</span>
                                            <span className={styles.docMeta}>
                                                {kb.file_count} 个文件 · {kb.chunk_count} 个知识块
                                            </span>
                                        </div>
                                        {isBound ? (
                                            <span className={styles.statusBadge} style={{ background: '#e0f2fe', color: '#0284c7' }}>已绑定</span>
                                        ) : (
                                            <span className={styles.statusBadge} style={{ background: '#f1f5f9', color: '#64748b' }}>未绑定</span>
                                        )}
                                    </div>
                                    <div className={styles.docActions} style={{ marginTop: '12px', justifyContent: 'flex-end' }}>
                                        {isBound ? (
                                            <button
                                                className={styles.deleteBtn}
                                                onClick={() => handleUnbindKb(ref.id)}
                                                disabled={unbindingRefId === ref.id}
                                            >
                                                {unbindingRefId === ref.id ? '解绑中...' : '取消绑定'}
                                            </button>
                                        ) : (
                                            <button
                                                className={styles.retryBtn}
                                                style={{ background: 'var(--color-primary)', color: 'white', border: 'none' }}
                                                onClick={() => handleBindKb(kb.id)}
                                                disabled={bindingKbId === kb.id}
                                            >
                                                {bindingKbId === kb.id ? '绑定中...' : '绑定到课程'}
                                            </button>
                                        )}
                                    </div>
                                </div>
                            );
                        })}
                    </div>
                </div>
            )}


        </div>
    );
}
