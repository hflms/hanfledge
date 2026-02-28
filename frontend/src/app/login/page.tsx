'use client';

import { useState, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { login, setToken } from '@/lib/api';
import styles from './page.module.css';

export default function LoginPage() {
    const router = useRouter();
    const [phone, setPhone] = useState('');
    const [password, setPassword] = useState('');
    const [error, setError] = useState('');
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e: FormEvent) => {
        e.preventDefault();
        setError('');
        setLoading(true);

        try {
            const res = await login(phone, password);
            setToken(res.token);

            // Route based on user role
            const roles = res.user.school_roles || [];
            const roleNames = roles.map(r => r.role?.name || '');

            if (roleNames.includes('SYS_ADMIN') || roleNames.includes('SCHOOL_ADMIN') || roleNames.includes('TEACHER')) {
                router.push('/teacher/courses');
            } else {
                router.push('/student/activities');
            }
        } catch (err: unknown) {
            setError(err instanceof Error ? err.message : '登录失败');
        } finally {
            setLoading(false);
        }
    };

    return (
        <div className={styles.loginPage}>
            <div className={styles.loginCard}>
                <div className={styles.logo}>
                    <div className={styles.logoIcon}>🎓</div>
                    <div className={styles.logoTitle}>Hanfledge</div>
                    <div className={styles.logoSub}>AI 智适应学习平台</div>
                </div>

                {error && <div className={styles.error}>{error}</div>}

                <form onSubmit={handleSubmit}>
                    <div className="form-group">
                        <label className="label" htmlFor="phone">手机号</label>
                        <input
                            id="phone"
                            className="input"
                            type="tel"
                            placeholder="请输入手机号"
                            value={phone}
                            onChange={e => setPhone(e.target.value)}
                            required
                            autoFocus
                        />
                    </div>

                    <div className="form-group">
                        <label className="label" htmlFor="password">密码</label>
                        <input
                            id="password"
                            className="input"
                            type="password"
                            placeholder="请输入密码"
                            value={password}
                            onChange={e => setPassword(e.target.value)}
                            required
                        />
                    </div>

                    <button
                        type="submit"
                        className={`btn btn-primary ${styles.submitBtn}`}
                        disabled={loading || !phone || !password}
                    >
                        {loading ? <span className="spinner" /> : null}
                        {loading ? '登录中...' : '登 录'}
                    </button>
                </form>

                <div className={styles.footer}>
                    Powered by Knowledge-Augmented RAG + Multi-Agent Orchestration
                </div>
            </div>
        </div>
    );
}
