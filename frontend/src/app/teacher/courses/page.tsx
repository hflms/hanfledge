'use client';

import { useEffect, useState, useCallback, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { listCourses, createCourse, type Course } from '@/lib/api';
import { COURSE_STATUS_MAP } from '@/lib/constants';
import { useModalA11y, cardA11yProps } from '@/lib/a11y';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

export default function CoursesPage() {
    const router = useRouter();
    const [courses, setCourses] = useState<Course[]>([]);
    const [loading, setLoading] = useState(true);
    const [showModal, setShowModal] = useState(false);
    const closeModal = useCallback(() => setShowModal(false), []);
    const modalRef = useModalA11y(showModal, closeModal);

    // Create form state
    const [form, setForm] = useState({
        title: '',
        subject: '',
        grade_level: 10,
        description: '',
    });
    const [creating, setCreating] = useState(false);

    const fetchCourses = async () => {
        try {
            const data = await listCourses();
            setCourses(data?.items || []);
        } catch (err) {
            console.error('Failed to fetch courses', err);
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        fetchCourses();
    }, []);

    const handleCreate = async (e: FormEvent) => {
        e.preventDefault();
        setCreating(true);
        try {
            await createCourse({
                school_id: 1, // TODO: select from user's schools
                ...form,
            });
            setShowModal(false);
            setForm({ title: '', subject: '', grade_level: 10, description: '' });
            fetchCourses();
        } catch (err) {
            console.error('Create failed', err);
        } finally {
            setCreating(false);
        }
    };

    if (loading) {
        return (
            <LoadingSpinner />
        );
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <h1 className={styles.pageTitle}>我的课程</h1>
                <button className="btn btn-primary" onClick={() => setShowModal(true)}>
                    ＋ 创建课程
                </button>
            </div>

            {courses.length === 0 ? (
                <div className={styles.emptyState}>
                    <div className={styles.emptyIcon}>📚</div>
                    <div className={styles.emptyText}>还没有课程，点击上方按钮创建第一个课程</div>
                </div>
            ) : (
                <div className={styles.courseGrid}>
                    {courses.map(course => (
                        <div
                            key={course.id}
                            className={`card ${styles.courseCard}`}
                            onClick={() => router.push(`/teacher/courses/${course.id}/outline`)}
                            {...cardA11yProps}
                        >
                            <div className={styles.courseCardTop}>
                                <div>
                                    <div className={styles.courseTitle}>{course.title}</div>
                                    <div className={styles.courseSubject}>{course.subject} · {course.grade_level}年级</div>
                                </div>
                                <span className={`badge badge-${course.status}`}>
                                    {COURSE_STATUS_MAP[course.status] || course.status}
                                </span>
                            </div>
                            {course.description && (
                                <div style={{ fontSize: 13, color: 'var(--text-muted)' }}>{course.description}</div>
                            )}
                            <div className={styles.courseMeta}>
                                <div className={styles.metaItem}>
                                    章节 <span className={styles.metaValue}>{course.chapters?.length || 0}</span>
                                </div>
                                <div className={styles.metaItem}>
                                    创建于 <span className={styles.metaValue}>{new Date(course.created_at).toLocaleDateString('zh-CN')}</span>
                                </div>
                            </div>
                        </div>
                    ))}
                </div>
            )}

            {/* Create Modal */}
            {showModal && (
                <div className={styles.modalOverlay} onClick={closeModal}>
                    <div className={styles.modal} onClick={e => e.stopPropagation()}
                         ref={modalRef} role="dialog" aria-modal="true" aria-labelledby="create-course-title" tabIndex={-1}>
                        <h2 className={styles.modalTitle} id="create-course-title">创建新课程</h2>
                        <form onSubmit={handleCreate}>
                            <div className="form-group">
                                <label className="label" htmlFor="title">课程名称</label>
                                <input id="title" className="input" placeholder="例：高一数学（上）"
                                    value={form.title} onChange={e => setForm({ ...form, title: e.target.value })} required />
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="subject">学科</label>
                                <input id="subject" className="input" placeholder="例：数学"
                                    value={form.subject} onChange={e => setForm({ ...form, subject: e.target.value })} required />
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="grade">年级</label>
                                <input id="grade" className="input" type="number" min={1} max={12}
                                    value={form.grade_level} onChange={e => setForm({ ...form, grade_level: parseInt(e.target.value) })} required />
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="desc">课程描述</label>
                                <input id="desc" className="input" placeholder="简要描述课程内容"
                                    value={form.description} onChange={e => setForm({ ...form, description: e.target.value })} />
                            </div>
                            <div className={styles.modalActions}>
                                <button type="button" className="btn btn-secondary" onClick={closeModal}>取消</button>
                                <button type="submit" className="btn btn-primary" disabled={creating}>
                                    {creating ? '创建中...' : '创建'}
                                </button>
                            </div>
                        </form>
                    </div>
                </div>
            )}
        </div>
    );
}
