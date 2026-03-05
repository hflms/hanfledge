// -- Document Status Labels -----------------------------------

/** Document processing status (uploaded → processing → completed/failed) */
export const DOCUMENT_STATUS_LABEL: Record<string, string> = {
    uploaded: '已上传',
    processing: '处理中...',
    completed: '已完成',
    failed: '处理失败',
};

export const DOCUMENT_STATUS_ICON: Record<string, string> = {
    uploaded: '📄',
    processing: '⏳',
    completed: '✅',
    failed: '❌',
};

// -- Course Status Labels -------------------------------------

/** Course lifecycle status */
export const COURSE_STATUS_MAP: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    archived: '已归档',
};

// -- Activity / Dashboard Status Labels -----------------------

/** Combined status map for activities & dashboard (superset of course statuses) */
export const ACTIVITY_STATUS_MAP: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    closed: '已关闭',
    active: '进行中',
    completed: '已完成',
    abandoned: '已放弃',
};

/** Student session status */
export const SESSION_STATUS_MAP: Record<string, string> = {
    active: '学习中',
    completed: '已完成',
    abandoned: '已放弃',
};

// -- Custom Skill Status Labels -------------------------------

import type { CustomSkillStatus } from '@/lib/api';

export const CUSTOM_SKILL_STATUS_LABELS: Record<CustomSkillStatus, string> = {
    draft: '草稿',
    published: '已发布',
    shared: '已共享',
    archived: '已归档',
};

// -- Skill Categories & Subjects ------------------------------

/** Teaching skill category names */
export const CATEGORY_MAP: Record<string, string> = {
    'inquiry-based': '探究式教学',
    'critical-thinking': '批判性思维',
    'collaborative': '协作学习',
    'role-play': '角色扮演',
    'content-creation': '内容创作',
};

/** Category emoji icons */
export const CATEGORY_ICONS: Record<string, string> = {
    'inquiry-based': '🔍',
    'critical-thinking': '🧐',
    'collaborative': '🤝',
    'role-play': '🎭',
    'content-creation': '🎨',
};

/** Academic subject names */
export const SUBJECT_MAP: Record<string, string> = {
    math: '数学',
    physics: '物理',
    chemistry: '化学',
    biology: '生物',
    chinese: '语文',
    english: '英语',
    history: '历史',
    geography: '地理',
};

// -- Scaffold Level Labels ------------------------------------

/** Scaffold assistance level labels */
export const SCAFFOLD_MAP: Record<string, string> = {
    high: '高',
    medium: '中',
    low: '低',
};
