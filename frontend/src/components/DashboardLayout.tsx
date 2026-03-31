'use client';

import { useEffect, ReactNode } from 'react';
import { useRouter, usePathname } from 'next/navigation';
import Link from 'next/link';
import { useAuthStore } from '@/lib/auth-store';
import { PluginRegistryProvider } from '@/lib/plugin/PluginRegistry';
import LoadingSpinner from '@/components/LoadingSpinner';
import NotificationBell from '@/components/NotificationBell';
import ThemeToggle from '@/components/ThemeToggle';
import styles from './DashboardLayout.module.css';

interface NavItem {
    icon: string;
    label: string;
    href: string;
}

const TEACHER_NAV: NavItem[] = [
    { icon: '📚', label: '课程管理', href: '/teacher/courses' },
    { icon: '🧠', label: 'WeKnora 知识库', href: '/teacher/weknora' },
    { icon: '🧩', label: '技能商店', href: '/teacher/skills' },
    { icon: '📊', label: '学情仪表盘', href: '/teacher/dashboard' },
    { icon: '⚙️', label: '系统设置', href: '/teacher/settings' },
];

const STUDENT_NAV: NavItem[] = [
    { icon: '📋', label: '学习活动', href: '/student/activities' },
    { icon: '📈', label: '我的掌握度', href: '/student/mastery' },
    { icon: '🏆', label: '我的成就', href: '/student/achievements' },
    { icon: '🗺️', label: '知识图谱', href: '/student/knowledge-map' },
    { icon: '📝', label: '错题本', href: '/student/error-notebook' },
];

const ADMIN_NAV: NavItem[] = [
    { icon: '🧭', label: '管理总览', href: '/admin/overview' },
    { icon: '🏫', label: '学校管理', href: '/admin/schools' },
    { icon: '🏷️', label: '班级管理', href: '/admin/classes' },
    { icon: '👥', label: '账号管理', href: '/admin/users' },
    { icon: '🧠', label: 'Soul 规则', href: '/admin/soul' },
];

const ROLE_LABELS: Record<string, string> = {
    SYS_ADMIN: '系统管理员',
    SCHOOL_ADMIN: '学校管理员',
    TEACHER: '教师',
    STUDENT: '学生',
};

interface DashboardLayoutProps {
    children: ReactNode;
    variant?: 'teacher' | 'student' | 'admin';
}

export default function DashboardLayout({ children, variant }: DashboardLayoutProps) {
    const router = useRouter();
    const pathname = usePathname();
    const { user, loading, fetchUser, logout } = useAuthStore();

    // Auto-detect variant from pathname if not provided
    const layoutVariant = variant || (
        pathname.startsWith('/admin')
            ? 'admin'
            : pathname.startsWith('/student')
                ? 'student'
                : 'teacher'
    );
    const roleNames = user?.school_roles?.map(r => r.role?.name).filter(Boolean) || [];
    const isAdmin = roleNames.includes('SYS_ADMIN') || roleNames.includes('SCHOOL_ADMIN');
    const isStudent = layoutVariant === 'student';
    const navItems = isStudent
        ? STUDENT_NAV
        : isAdmin
            ? [...ADMIN_NAV, ...TEACHER_NAV]
            : TEACHER_NAV;
    const sectionLabel = isStudent
        ? '学习中心'
        : isAdmin
            ? '系统管理与教学'
            : '教学管理';

    useEffect(() => {
        fetchUser().then(() => {
            // Redirect to login if user couldn't be loaded (token expired/invalid)
            if (!useAuthStore.getState().user) {
                router.push('/login');
            }
        });
    }, [fetchUser, router]);

    const handleLogout = () => {
        logout();
        router.push('/login');
    };

    const primaryRole = user?.school_roles?.[0]?.role?.name || (layoutVariant === 'student' ? 'STUDENT' : 'TEACHER');

    if (loading || !user) {
        return (
            <LoadingSpinner size="fullscreen" />
        );
    }

    return (
        <PluginRegistryProvider>
            <div className={styles.layoutWrapper}>
                {/* Skip to content link for keyboard/screen-reader users */}
                <a href="#main-content" className={styles.skipLink}>
                    跳转到主要内容
                </a>

                {/* Sidebar */}
                <aside className={styles.sidebar}>
                    <div className={styles.sidebarBrand}>
                        <div className={styles.brandIcon}>🎓</div>
                        <div className={styles.brandName}>Hanfledge</div>
                    </div>

                    <nav className={styles.sidebarNav}>
                        <div className={styles.navSection}>
                            <div className={styles.navLabel}>{sectionLabel}</div>
                            {navItems.map(item => (
                                <Link
                                    key={item.href}
                                    href={item.href}
                                    className={`${styles.navItem} ${pathname.startsWith(item.href) ? styles.navItemActive : ''}`}
                                >
                                    <span className={styles.navItemIcon}>{item.icon}</span>
                                    {item.label}
                                </Link>
                            ))}
                        </div>
                    </nav>

                    <div className={styles.sidebarFooter}>
                        <Link href="/help" className={styles.helpLink}>
                            <span className={styles.navItemIcon}>❓</span>
                            帮助文档
                        </Link>
                    </div>
                </aside>

                {/* Main */}
                <div className={styles.mainArea}>
                    <header className={styles.header}>
                        <div className={styles.headerTitle}>
                            {navItems.find(n => pathname.startsWith(n.href))?.label || 'Hanfledge'}
                        </div>
                        <div className={styles.headerRight}>
                            <ThemeToggle />
                            {isAdmin && <NotificationBell />}
                            <div className={styles.userInfo}>
                                <span className={styles.userName}>{user.display_name}</span>
                                <span className={styles.userRole}>{ROLE_LABELS[primaryRole] || primaryRole}</span>
                            </div>
                            <button className={styles.logoutBtn} onClick={handleLogout}>
                                退出
                            </button>
                        </div>
                    </header>

                    <main id="main-content" className={styles.content}>
                        {children}
                    </main>
                </div>
            </div>
        </PluginRegistryProvider>
    );
}
