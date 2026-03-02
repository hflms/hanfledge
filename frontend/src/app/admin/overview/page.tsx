'use client';

import { useMemo } from 'react';
import Link from 'next/link';
import { useApi } from '@/lib/useApi';
import {
    type PaginatedResponse,
    type School,
    type ClassItem,
    type AdminUser,
    type Course,
    type LearningActivity,
} from '@/lib/api';
import LoadingSpinner from '@/components/LoadingSpinner';
import styles from './page.module.css';

export default function AdminOverviewPage() {
    const { data: schoolsData, isLoading: loadingSchools } = useApi<PaginatedResponse<School>>('/schools');
    const { data: classesData, isLoading: loadingClasses } = useApi<PaginatedResponse<ClassItem>>('/classes');
    const { data: usersData, isLoading: loadingUsers } = useApi<PaginatedResponse<AdminUser>>('/users');
    const { data: coursesData, isLoading: loadingCourses } = useApi<PaginatedResponse<Course>>('/courses');
    const { data: activitiesData, isLoading: loadingActivities } = useApi<PaginatedResponse<LearningActivity>>('/activities');

    const loading = loadingSchools || loadingClasses || loadingUsers || loadingCourses || loadingActivities;

    const stats = useMemo(() => {
        const activities = activitiesData?.items || [];
        return {
            schools: schoolsData?.total || 0,
            classes: classesData?.total || 0,
            users: usersData?.total || 0,
            courses: coursesData?.total || 0,
            activities: activities.length,
            publishedActivities: activities.filter(a => a.status === 'published').length,
        };
    }, [schoolsData, classesData, usersData, coursesData, activitiesData]);

    if (loading) {
        return <LoadingSpinner />;
    }

    return (
        <div className="fade-in">
            <div className={styles.pageHeader}>
                <div>
                    <h1 className={styles.pageTitle}>管理总览</h1>
                    <p className={styles.pageSubtitle}>快速查看全局数据并进入管理与教学入口</p>
                </div>
                <div className={styles.timeLabel}>今日概览</div>
            </div>

            <div className={styles.statsGrid}>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>学校</div>
                    <div className={styles.statValue}>{stats.schools}</div>
                    <div className={styles.statHint}>已接入学校数量</div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>班级</div>
                    <div className={styles.statValue}>{stats.classes}</div>
                    <div className={styles.statHint}>当前教学班级数</div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>账号</div>
                    <div className={styles.statValue}>{stats.users}</div>
                    <div className={styles.statHint}>师生账号总量</div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>课程</div>
                    <div className={styles.statValue}>{stats.courses}</div>
                    <div className={styles.statHint}>创建的课程总数</div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>学习活动</div>
                    <div className={styles.statValue}>{stats.activities}</div>
                    <div className={styles.statHint}>已创建活动数量</div>
                </div>
                <div className={styles.statCard}>
                    <div className={styles.statLabel}>已发布</div>
                    <div className={styles.statValue}>{stats.publishedActivities}</div>
                    <div className={styles.statHint}>学生可见活动数</div>
                </div>
            </div>

            <div className={styles.quickLinks}>
                <div className={styles.sectionTitle}>管理入口</div>
                <div className={styles.linkGrid}>
                    <Link href="/admin/schools" className={styles.linkCard}>
                        <div className={styles.linkIcon}>🏫</div>
                        <div>
                            <div className={styles.linkTitle}>学校管理</div>
                            <div className={styles.linkHint}>创建与维护学校</div>
                        </div>
                    </Link>
                    <Link href="/admin/classes" className={styles.linkCard}>
                        <div className={styles.linkIcon}>🏷️</div>
                        <div>
                            <div className={styles.linkTitle}>班级管理</div>
                            <div className={styles.linkHint}>管理教学班级</div>
                        </div>
                    </Link>
                    <Link href="/admin/users" className={styles.linkCard}>
                        <div className={styles.linkIcon}>👥</div>
                        <div>
                            <div className={styles.linkTitle}>账号管理</div>
                            <div className={styles.linkHint}>创建师生账号</div>
                        </div>
                    </Link>
                </div>
            </div>

            <div className={styles.quickLinks}>
                <div className={styles.sectionTitle}>教学入口</div>
                <div className={styles.linkGrid}>
                    <Link href="/teacher/courses" className={styles.linkCard}>
                        <div className={styles.linkIcon}>📚</div>
                        <div>
                            <div className={styles.linkTitle}>课程管理</div>
                            <div className={styles.linkHint}>创建课程与上传教材</div>
                        </div>
                    </Link>
                    <Link href="/teacher/dashboard" className={styles.linkCard}>
                        <div className={styles.linkIcon}>📊</div>
                        <div>
                            <div className={styles.linkTitle}>学情仪表盘</div>
                            <div className={styles.linkHint}>查看教学分析数据</div>
                        </div>
                    </Link>
                    <Link href="/teacher/skills" className={styles.linkCard}>
                        <div className={styles.linkIcon}>🧩</div>
                        <div>
                            <div className={styles.linkTitle}>技能商店</div>
                            <div className={styles.linkHint}>管理教学技能</div>
                        </div>
                    </Link>
                </div>
            </div>
        </div>
    );
}
