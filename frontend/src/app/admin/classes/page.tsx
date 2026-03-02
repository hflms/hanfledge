'use client';

import { useEffect, useMemo, useState } from 'react';
import { createClass, listClasses, listSchools, type ClassItem, type School } from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

const DEFAULT_PAGE_SIZE = 20;

export default function AdminClassesPage() {
    const { toast } = useToast();
    const [classes, setClasses] = useState<ClassItem[]>([]);
    const [schools, setSchools] = useState<School[]>([]);
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);

    const [name, setName] = useState('');
    const [gradeLevel, setGradeLevel] = useState('1');
    const [schoolId, setSchoolId] = useState('');

    const totalPages = useMemo(() => Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE)), [total]);

    const loadSchools = async () => {
        try {
            const res = await listSchools({ page: 1, limit: 200 });
            setSchools(res.items);
            if (!schoolId && res.items.length > 0) {
                setSchoolId(String(res.items[0].id));
            }
        } catch (err) {
            console.error('Failed to load schools:', err);
        }
    };

    const loadClasses = async (nextPage = 1) => {
        setLoading(true);
        try {
            const res = await listClasses({ page: nextPage, limit: DEFAULT_PAGE_SIZE });
            setClasses(res.items);
            setTotal(res.total);
            setPage(res.page);
        } catch (err) {
            console.error('Failed to load classes:', err);
            toast('加载班级列表失败', 'error');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        loadSchools();
        loadClasses(1);
    }, []);

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !gradeLevel || !schoolId) {
            toast('请填写完整信息', 'warning');
            return;
        }
        setSubmitting(true);
        try {
            await createClass({
                name,
                grade_level: Number(gradeLevel),
                school_id: Number(schoolId),
            });
            setName('');
            toast('班级创建成功', 'success');
            loadClasses(1);
        } catch (err) {
            console.error('Failed to create class:', err);
            toast('班级创建失败', 'error');
        } finally {
            setSubmitting(false);
        }
    };

    if (loading) {
        return <LoadingSpinner size="large" />;
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <div>
                    <h1 className={styles.pageTitle}>班级管理</h1>
                    <p className={styles.pageSubtitle}>为学校创建并维护教学班级</p>
                </div>
                <div className={styles.pageMeta}>共 {total} 个班级</div>
            </div>

            <div className={`card ${styles.formCard}`}>
                <div className={styles.cardTitle}>新建班级</div>
                <form className={styles.form} onSubmit={handleCreate}>
                    <div className={styles.formRow}>
                        <div className="form-group">
                            <label className="label" htmlFor="class-name">班级名称</label>
                            <input
                                id="class-name"
                                className="input"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="例如：高一(1)班"
                                required
                            />
                        </div>
                        <div className="form-group">
                            <label className="label" htmlFor="class-grade">年级</label>
                            <input
                                id="class-grade"
                                className="input"
                                type="number"
                                min={1}
                                max={12}
                                value={gradeLevel}
                                onChange={(e) => setGradeLevel(e.target.value)}
                                required
                            />
                        </div>
                        <div className="form-group">
                            <label className="label" htmlFor="class-school">所属学校</label>
                            <select
                                id="class-school"
                                className="input"
                                value={schoolId}
                                onChange={(e) => setSchoolId(e.target.value)}
                                required
                            >
                                {schools.map((school) => (
                                    <option key={school.id} value={school.id}>
                                        {school.name}
                                    </option>
                                ))}
                            </select>
                        </div>
                    </div>
                    <button
                        type="submit"
                        className="btn btn-primary"
                        disabled={submitting}
                    >
                        {submitting ? '创建中...' : '创建班级'}
                    </button>
                </form>
            </div>

            <div className={styles.listSection}>
                <div className={styles.sectionTitle}>班级列表</div>
                {classes.length === 0 ? (
                    <div className={styles.emptyState}>暂无班级数据</div>
                ) : (
                    <div className={styles.tableWrapper}>
                        <table className={styles.table}>
                            <thead>
                                <tr>
                                    <th>班级名称</th>
                                    <th>年级</th>
                                    <th>所属学校</th>
                                    <th>创建时间</th>
                                </tr>
                            </thead>
                            <tbody>
                                {classes.map((item) => (
                                    <tr key={item.id}>
                                        <td>{item.name}</td>
                                        <td>{item.grade_level}</td>
                                        <td>{schools.find((s) => s.id === item.school_id)?.name || `#${item.school_id}`}</td>
                                        <td>{new Date(item.created_at).toLocaleDateString('zh-CN')}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}

                <div className={styles.pagination}>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadClasses(Math.max(1, page - 1))}
                        disabled={page <= 1}
                    >
                        上一页
                    </button>
                    <span className={styles.pageInfo}>第 {page} / {totalPages} 页</span>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadClasses(Math.min(totalPages, page + 1))}
                        disabled={page >= totalPages}
                    >
                        下一页
                    </button>
                </div>
            </div>
        </div>
    );
}
