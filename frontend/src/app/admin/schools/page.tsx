'use client';

import { useEffect, useMemo, useState } from 'react';
import { createSchool, listSchools, type School } from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

const DEFAULT_PAGE_SIZE = 20;

export default function AdminSchoolsPage() {
    const { toast } = useToast();
    const [schools, setSchools] = useState<School[]>([]);
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);

    const [name, setName] = useState('');
    const [code, setCode] = useState('');
    const [city, setCity] = useState('');

    const totalPages = useMemo(() => Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE)), [total]);

    const loadSchools = async (nextPage = 1) => {
        setLoading(true);
        try {
            const res = await listSchools({ page: nextPage, limit: DEFAULT_PAGE_SIZE });
            setSchools(res.items);
            setTotal(res.total);
            setPage(res.page);
        } catch (err) {
            console.error('Failed to load schools:', err);
            toast('加载学校列表失败', 'error');
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        loadSchools(1);
    }, []);

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !code || !city) {
            toast('请填写完整信息', 'warning');
            return;
        }
        setSubmitting(true);
        try {
            await createSchool({ name, code, city });
            setName('');
            setCode('');
            setCity('');
            toast('学校创建成功', 'success');
            loadSchools(1);
        } catch (err) {
            console.error('Failed to create school:', err);
            toast('学校创建失败', 'error');
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
                    <h1 className={styles.pageTitle}>学校管理</h1>
                    <p className={styles.pageSubtitle}>创建与维护平台学校账号</p>
                </div>
                <div className={styles.pageMeta}>共 {total} 所学校</div>
            </div>

            <div className={`card ${styles.formCard}`}>
                <div className={styles.cardTitle}>新建学校</div>
                <form className={styles.form} onSubmit={handleCreate}>
                    <div className={styles.formRow}>
                        <div className="form-group">
                            <label className="label" htmlFor="school-name">学校名称</label>
                            <input
                                id="school-name"
                                className="input"
                                value={name}
                                onChange={(e) => setName(e.target.value)}
                                placeholder="例如：恒星中学"
                                required
                            />
                        </div>
                        <div className="form-group">
                            <label className="label" htmlFor="school-code">学校代码</label>
                            <input
                                id="school-code"
                                className="input"
                                value={code}
                                onChange={(e) => setCode(e.target.value)}
                                placeholder="例如：HXZS"
                                required
                            />
                        </div>
                        <div className="form-group">
                            <label className="label" htmlFor="school-city">所在城市</label>
                            <input
                                id="school-city"
                                className="input"
                                value={city}
                                onChange={(e) => setCity(e.target.value)}
                                placeholder="例如：上海"
                                required
                            />
                        </div>
                    </div>
                    <button
                        type="submit"
                        className="btn btn-primary"
                        disabled={submitting}
                    >
                        {submitting ? '创建中...' : '创建学校'}
                    </button>
                </form>
            </div>

            <div className={styles.listSection}>
                <div className={styles.sectionTitle}>学校列表</div>
                {schools.length === 0 ? (
                    <div className={styles.emptyState}>暂无学校数据</div>
                ) : (
                    <div className={styles.tableWrapper}>
                        <table className={styles.table}>
                            <thead>
                                <tr>
                                    <th>学校名称</th>
                                    <th>代码</th>
                                    <th>城市</th>
                                    <th>状态</th>
                                    <th>创建时间</th>
                                </tr>
                            </thead>
                            <tbody>
                                {schools.map((school) => (
                                    <tr key={school.id}>
                                        <td>{school.name}</td>
                                        <td>{school.code}</td>
                                        <td>{school.city}</td>
                                        <td>{school.status}</td>
                                        <td>{new Date(school.created_at).toLocaleDateString('zh-CN')}</td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                    </div>
                )}

                <div className={styles.pagination}>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadSchools(Math.max(1, page - 1))}
                        disabled={page <= 1}
                    >
                        上一页
                    </button>
                    <span className={styles.pageInfo}>第 {page} / {totalPages} 页</span>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadSchools(Math.min(totalPages, page + 1))}
                        disabled={page >= totalPages}
                    >
                        下一页
                    </button>
                </div>
            </div>
        </div>
    );
}
