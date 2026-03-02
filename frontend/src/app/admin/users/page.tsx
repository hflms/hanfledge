'use client';

import { useEffect, useMemo, useState } from 'react';
import {
    batchCreateUsers,
    createAdminUser,
    listAdminUsers,
    listClasses,
    listSchools,
    type AdminUser,
    type ClassItem,
    type School,
} from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import { useToast } from '@/components/Toast';
import styles from './page.module.css';

const DEFAULT_PAGE_SIZE = 20;

const ROLE_OPTIONS = [
    { value: 'SCHOOL_ADMIN', label: '学校管理员' },
    { value: 'TEACHER', label: '教师' },
    { value: 'STUDENT', label: '学生' },
];

export default function AdminUsersPage() {
    const { toast } = useToast();
    const [users, setUsers] = useState<AdminUser[]>([]);
    const [schools, setSchools] = useState<School[]>([]);
    const [classes, setClasses] = useState<ClassItem[]>([]);
    const [loading, setLoading] = useState(true);
    const [submitting, setSubmitting] = useState(false);
    const [page, setPage] = useState(1);
    const [total, setTotal] = useState(0);

    const [name, setName] = useState('');
    const [phone, setPhone] = useState('');
    const [password, setPassword] = useState('');
    const [roleName, setRoleName] = useState('STUDENT');
    const [schoolId, setSchoolId] = useState('');
    const [classId, setClassId] = useState('');

    const [batchText, setBatchText] = useState('');
    const [batchRole, setBatchRole] = useState('STUDENT');
    const [batchSchoolId, setBatchSchoolId] = useState('');
    const [batchClassId, setBatchClassId] = useState('');

    const totalPages = useMemo(() => Math.max(1, Math.ceil(total / DEFAULT_PAGE_SIZE)), [total]);

    const loadUsers = async (nextPage = 1) => {
        setLoading(true);
        try {
            const res = await listAdminUsers({ page: nextPage, limit: DEFAULT_PAGE_SIZE });
            setUsers(res.items);
            setTotal(res.total);
            setPage(res.page);
        } catch (err) {
            console.error('Failed to load users:', err);
            toast('加载账号列表失败', 'error');
        } finally {
            setLoading(false);
        }
    };

    const loadDeps = async () => {
        try {
            const [schoolRes, classRes] = await Promise.all([
                listSchools({ page: 1, limit: 200 }),
                listClasses({ page: 1, limit: 200 }),
            ]);
            setSchools(schoolRes.items);
            setClasses(classRes.items);
            if (!schoolId && schoolRes.items.length > 0) {
                setSchoolId(String(schoolRes.items[0].id));
            }
            if (!batchSchoolId && schoolRes.items.length > 0) {
                setBatchSchoolId(String(schoolRes.items[0].id));
            }
        } catch (err) {
            console.error('Failed to load admin dependencies:', err);
        }
    };

    useEffect(() => {
        loadDeps();
        loadUsers(1);
    }, []);

    const handleCreate = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !phone || !password || !schoolId) {
            toast('请填写完整信息', 'warning');
            return;
        }
        setSubmitting(true);
        try {
            await createAdminUser({
                phone,
                password,
                display_name: name,
                role_name: roleName,
                school_id: Number(schoolId),
                class_id: classId ? Number(classId) : undefined,
            });
            setName('');
            setPhone('');
            setPassword('');
            setClassId('');
            toast('账号创建成功', 'success');
            loadUsers(1);
        } catch (err) {
            console.error('Failed to create user:', err);
            toast('账号创建失败', 'error');
        } finally {
            setSubmitting(false);
        }
    };

    const parseBatchUsers = () => {
        const lines = batchText.split('\n').map(line => line.trim()).filter(Boolean);
        return lines.map((line) => {
            const [namePart, phonePart] = line.split(',').map(part => part.trim());
            return { name: namePart, phone: phonePart };
        }).filter(item => item.name && item.phone);
    };

    const handleBatch = async (e: React.FormEvent) => {
        e.preventDefault();
        const users = parseBatchUsers();
        if (!batchSchoolId || users.length === 0) {
            toast('请填写学校并输入用户列表', 'warning');
            return;
        }
        setSubmitting(true);
        try {
            const res = await batchCreateUsers({
                school_id: Number(batchSchoolId),
                class_id: batchClassId ? Number(batchClassId) : undefined,
                role_name: batchRole,
                users,
            });
            setBatchText('');
            setBatchClassId('');
            toast(`批量创建成功，共 ${res.count} 个账号`, 'success');
            loadUsers(1);
        } catch (err) {
            console.error('Failed to batch create users:', err);
            toast('批量创建失败', 'error');
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
                    <h1 className={styles.pageTitle}>账号管理</h1>
                    <p className={styles.pageSubtitle}>创建学生与教师账号，并绑定学校与班级</p>
                </div>
                <div className={styles.pageMeta}>共 {total} 个账号</div>
            </div>

            <div className={styles.formsGrid}>
                <div className={`card ${styles.formCard}`}>
                    <div className={styles.cardTitle}>单个创建</div>
                    <form className={styles.form} onSubmit={handleCreate}>
                        <div className={styles.formRow}>
                            <div className="form-group">
                                <label className="label" htmlFor="user-name">姓名</label>
                                <input
                                    id="user-name"
                                    className="input"
                                    value={name}
                                    onChange={(e) => setName(e.target.value)}
                                    placeholder="例如：张晓宇"
                                    required
                                />
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="user-phone">手机号</label>
                                <input
                                    id="user-phone"
                                    className="input"
                                    value={phone}
                                    onChange={(e) => setPhone(e.target.value)}
                                    placeholder="请输入手机号"
                                    required
                                />
                            </div>
                        </div>
                        <div className={styles.formRow}>
                            <div className="form-group">
                                <label className="label" htmlFor="user-password">初始密码</label>
                                <input
                                    id="user-password"
                                    className="input"
                                    value={password}
                                    onChange={(e) => setPassword(e.target.value)}
                                    placeholder="请输入初始密码"
                                    required
                                />
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="user-role">角色</label>
                                <select
                                    id="user-role"
                                    className="input"
                                    value={roleName}
                                    onChange={(e) => setRoleName(e.target.value)}
                                    required
                                >
                                    {ROLE_OPTIONS.map((role) => (
                                        <option key={role.value} value={role.value}>{role.label}</option>
                                    ))}
                                </select>
                            </div>
                        </div>
                        <div className={styles.formRow}>
                            <div className="form-group">
                                <label className="label" htmlFor="user-school">所属学校</label>
                                <select
                                    id="user-school"
                                    className="input"
                                    value={schoolId}
                                    onChange={(e) => setSchoolId(e.target.value)}
                                    required
                                >
                                    {schools.map((school) => (
                                        <option key={school.id} value={school.id}>{school.name}</option>
                                    ))}
                                </select>
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="user-class">所属班级 (可选)</label>
                                <select
                                    id="user-class"
                                    className="input"
                                    value={classId}
                                    onChange={(e) => setClassId(e.target.value)}
                                >
                                    <option value="">不选择</option>
                                    {classes.map((item) => (
                                        <option key={item.id} value={item.id}>{item.name}</option>
                                    ))}
                                </select>
                            </div>
                        </div>
                        <button
                            type="submit"
                            className="btn btn-primary"
                            disabled={submitting}
                        >
                            {submitting ? '创建中...' : '创建账号'}
                        </button>
                    </form>
                </div>

                <div className={`card ${styles.formCard}`}>
                    <div className={styles.cardTitle}>批量创建</div>
                    <form className={styles.form} onSubmit={handleBatch}>
                        <div className={styles.formRow}>
                            <div className="form-group">
                                <label className="label" htmlFor="batch-role">角色</label>
                                <select
                                    id="batch-role"
                                    className="input"
                                    value={batchRole}
                                    onChange={(e) => setBatchRole(e.target.value)}
                                    required
                                >
                                    {ROLE_OPTIONS.map((role) => (
                                        <option key={role.value} value={role.value}>{role.label}</option>
                                    ))}
                                </select>
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="batch-school">所属学校</label>
                                <select
                                    id="batch-school"
                                    className="input"
                                    value={batchSchoolId}
                                    onChange={(e) => setBatchSchoolId(e.target.value)}
                                    required
                                >
                                    {schools.map((school) => (
                                        <option key={school.id} value={school.id}>{school.name}</option>
                                    ))}
                                </select>
                            </div>
                        </div>
                        <div className={styles.formRow}>
                            <div className="form-group">
                                <label className="label" htmlFor="batch-class">所属班级 (可选)</label>
                                <select
                                    id="batch-class"
                                    className="input"
                                    value={batchClassId}
                                    onChange={(e) => setBatchClassId(e.target.value)}
                                >
                                    <option value="">不选择</option>
                                    {classes.map((item) => (
                                        <option key={item.id} value={item.id}>{item.name}</option>
                                    ))}
                                </select>
                            </div>
                            <div className="form-group">
                                <label className="label" htmlFor="batch-users">用户列表</label>
                                <textarea
                                    id="batch-users"
                                    className="input"
                                    value={batchText}
                                    onChange={(e) => setBatchText(e.target.value)}
                                    placeholder="每行一个用户，例如：张晓宇,13800001001"
                                    rows={5}
                                    required
                                />
                            </div>
                        </div>
                        <button
                            type="submit"
                            className="btn btn-primary"
                            disabled={submitting}
                        >
                            {submitting ? '创建中...' : '批量创建'}
                        </button>
                    </form>
                </div>
            </div>

            <div className={styles.listSection}>
                <div className={styles.sectionTitle}>账号列表</div>
                {users.length === 0 ? (
                    <div className={styles.emptyState}>暂无账号数据</div>
                ) : (
                    <div className={styles.tableWrapper}>
                        <table className={styles.table}>
                            <thead>
                                <tr>
                                    <th>姓名</th>
                                    <th>手机号</th>
                                    <th>角色</th>
                                    <th>学校</th>
                                    <th>状态</th>
                                </tr>
                            </thead>
                            <tbody>
                                {users.map((user) => {
                                    const role = user.school_roles?.[0];
                                    return (
                                        <tr key={user.id}>
                                            <td>{user.display_name}</td>
                                            <td>{user.phone}</td>
                                            <td>{role?.role?.name || '-'}</td>
                                            <td>{role?.school?.name || '-'}</td>
                                            <td>{user.status}</td>
                                        </tr>
                                    );
                                })}
                            </tbody>
                        </table>
                    </div>
                )}

                <div className={styles.pagination}>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadUsers(Math.max(1, page - 1))}
                        disabled={page <= 1}
                    >
                        上一页
                    </button>
                    <span className={styles.pageInfo}>第 {page} / {totalPages} 页</span>
                    <button
                        className="btn btn-secondary"
                        onClick={() => loadUsers(Math.min(totalPages, page + 1))}
                        disabled={page >= totalPages}
                    >
                        下一页
                    </button>
                </div>
            </div>
        </div>
    );
}
