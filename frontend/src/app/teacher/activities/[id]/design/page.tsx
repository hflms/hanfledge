'use client';

import { useEffect, useState, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';

import {
  getActivity,
  updateActivity,
  saveActivitySteps,
  publishActivity,
  listDesigners,
  type LearningActivity,
  type ActivityStep,
  type SaveStepData,
  type InstructionalDesigner,
} from '@/lib/api';
import { useToast } from '@/components/Toast';
import LoadingSpinner from '@/components/LoadingSpinner';
import AutonomousTab from './components/AutonomousTab';
import GuidedStepsTab from './components/GuidedStepsTab';

import styles from './page.module.css';

// -- Types --------------------------------------------------------

type TabType = 'autonomous' | 'guided';

// -- Helpers ------------------------------------------------------

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString('zh-CN', {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit',
  });
}

// -- Component ----------------------------------------------------

export default function ActivityDesignPage() {
  const params = useParams();
  const router = useRouter();
  const activityId = Number(params.id);
  const { toast } = useToast();

  // State
  const [activity, setActivity] = useState<LearningActivity | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [activeTab, setActiveTab] = useState<TabType>('autonomous');
  const [designers, setDesigners] = useState<InstructionalDesigner[]>([]);

  // Editable fields
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [designerId, setDesignerId] = useState('');
  const [deadline, setDeadline] = useState('');
  const [allowRetry, setAllowRetry] = useState(true);
  const [maxAttempts, setMaxAttempts] = useState(3);

  // Steps (for guided mode)
  const [steps, setSteps] = useState<SaveStepData[]>([]);
  const [dirty, setDirty] = useState(false);

  // -- Data Fetching -----------------------------------------------

  const fetchActivity = useCallback(async () => {
    try {
      const data = await getActivity(activityId);
      setActivity(data);
      setTitle(data.title);
      setDescription(data.description ?? '');
      setActiveTab(data.type);
      setDesignerId(data.designer_id ?? '');
      setDeadline(data.deadline ?? '');
      setAllowRetry(data.allow_retry);
      setMaxAttempts(data.max_attempts);

      // Map existing steps
      if (data.steps && data.steps.length > 0) {
        setSteps(data.steps.map((s: ActivityStep) => ({
          id: s.id,
          title: s.title,
          description: s.description,
          step_type: s.step_type,
          sort_order: s.sort_order,
          content_blocks: s.content_blocks,
          duration: s.duration,
        })));
      }
    } catch (err) {
      console.error('加载活动失败', err);
      toast('加载活动失败', 'error');
    } finally {
      setLoading(false);
    }
  }, [activityId, toast]);

  const fetchDesigners = useCallback(async () => {
    try {
      const data = await listDesigners();
      setDesigners(data);
    } catch (err) {
      console.error('加载设计者列表失败', err);
    }
  }, []);

  useEffect(() => { fetchActivity(); }, [fetchActivity]);
  useEffect(() => { fetchDesigners(); }, [fetchDesigners]);

  // -- Warn before leaving with unsaved changes -------------------

  useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener('beforeunload', handler);
    return () => window.removeEventListener('beforeunload', handler);
  }, [dirty]);

  // -- Handlers ----------------------------------------------------

  const handleSave = useCallback(async () => {
    if (!activity) return;
    setSaving(true);
    try {
      // Update activity metadata
      await updateActivity(activityId, {
        title,
        description: description || undefined,
        type: activeTab,
        designer_id: designerId || undefined,
        deadline: deadline || undefined,
        allow_retry: allowRetry,
        max_attempts: maxAttempts,
      });

      // Save steps if in guided mode
      if (activeTab === 'guided' && steps.length > 0) {
        const stepsWithOrder = steps.map((s, i) => ({
          ...s,
          sort_order: i,
        }));
        await saveActivitySteps(activityId, stepsWithOrder);
      }

      setDirty(false);
      toast('保存成功', 'success');

      // Reload to get fresh data
      await fetchActivity();
    } catch (err) {
      console.error('保存失败', err);
      toast('保存失败，请重试', 'error');
    } finally {
      setSaving(false);
    }
  }, [activity, activityId, title, description, activeTab, designerId, deadline, allowRetry, maxAttempts, steps, toast, fetchActivity]);

  const handlePublish = useCallback(async () => {
    if (!activity) return;

    if (activeTab === 'guided' && steps.length === 0) {
      toast('请至少添加一个学习环节', 'error');
      return;
    }

    // Save first if dirty
    if (dirty) {
      await handleSave();
    }

    setPublishing(true);
    try {
      await publishActivity(activityId);
      toast('活动已发布', 'success');
      router.push(`/teacher/courses/${activity.course_id}/outline`);
    } catch (err) {
      console.error('发布失败', err);
      toast('发布失败，请重试', 'error');
    } finally {
      setPublishing(false);
    }
  }, [activity, activityId, activeTab, steps, dirty, handleSave, toast, router]);

  const markDirty = useCallback(() => {
    setDirty(true);
  }, []);

  // -- Tab switch handler ------------------------------------------

  const handleTabChange = useCallback((tab: TabType) => {
    setActiveTab(tab);
    setDirty(true);
  }, []);

  // -- Loading guard -----------------------------------------------

  if (loading) return <LoadingSpinner />;

  if (!activity) {
    return (
      <div className={`fade-in ${styles.designPageWide}`}>
        <p>活动不存在或无权访问</p>
      </div>
    );
  }

  const isPublished = activity.status === 'published';

  return (
    <div className={`fade-in ${styles.designPageWide}`}>
      {/* Back link */}
      <a
        className={styles.backLink}
        href={`/teacher/courses/${activity.course_id}/outline`}
        onClick={(e) => {
          e.preventDefault();
          router.push(`/teacher/courses/${activity.course_id}/outline`);
        }}
      >
        &larr; 返回课程大纲
      </a>

      {/* Page Header */}
      <div className={styles.pageHeader}>
        <div className={styles.headerLeft}>
          <h1 className={styles.pageTitle}>
            <input
              className={`${styles.formInput} ${styles.titleInput}`}
              value={title}
              onChange={(e) => { setTitle(e.target.value); markDirty(); }}
              placeholder="活动标题…"
              disabled={isPublished}
              name="activity-title"
              aria-label="活动标题"
              autoComplete="off"
            />
          </h1>
          <p className={styles.pageSubtitle}>
            创建于 {formatDate(activity.created_at)}
            {activity.updated_at && ` · 更新于 ${formatDate(activity.updated_at)}`}
          </p>
        </div>
        <div className={styles.headerActions}>
          <span className={`${styles.statusBadge} ${isPublished ? styles.statusBadgePublished : styles.statusBadgeDraft}`}>
            {isPublished ? '已发布' : '草稿'}
          </span>
        </div>
      </div>

      {/* Description */}
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>活动描述</label>
        <textarea
          className={styles.formTextarea}
          value={description}
          onChange={(e) => { setDescription(e.target.value); markDirty(); }}
          placeholder="简要描述活动目标和要求…"
          disabled={isPublished}
          name="activity-description"
          rows={3}
        />
      </div>

      {/* Activity Type Tabs */}
      <div className={styles.tabs}>
        <button
          className={`${styles.tabBtn} ${activeTab === 'autonomous' ? styles.tabBtnActive : ''}`}
          onClick={() => handleTabChange('autonomous')}
          disabled={isPublished}
        >
          全自主学习
        </button>
        <button
          className={`${styles.tabBtn} ${activeTab === 'guided' ? styles.tabBtnActive : ''}`}
          onClick={() => handleTabChange('guided')}
          disabled={isPublished}
        >
          教师规定环节
        </button>
      </div>

      {/* Tab Content */}
      {activeTab === 'autonomous' ? (
        <AutonomousTab
          designers={designers}
          designerId={designerId}
          deadline={deadline}
          allowRetry={allowRetry}
          maxAttempts={maxAttempts}
          disabled={isPublished}
          onDesignerChange={(v) => { setDesignerId(v); markDirty(); }}
          onDeadlineChange={(v) => { setDeadline(v); markDirty(); }}
          onAllowRetryChange={(v) => { setAllowRetry(v); markDirty(); }}
          onMaxAttemptsChange={(v) => { setMaxAttempts(v); markDirty(); }}
        />
      ) : (
        <GuidedStepsTab
          activityId={activityId}
          activityTitle={title}
          steps={steps}
          disabled={isPublished}
          onStepsChange={(newSteps) => { setSteps(newSteps); markDirty(); }}
        />
      )}

      {/* Save Bar */}
      {!isPublished && (
        <div className={styles.saveBar}>
          <span className={styles.saveBarStatus} aria-live="polite">
            {dirty ? '有未保存的更改' : '所有更改已保存'}
          </span>
          <div className={styles.saveBarActions}>
            <button
              className={styles.btnSecondary}
              onClick={handleSave}
              disabled={saving || !dirty}
            >
              {saving ? '保存中\u2026' : '保存草稿'}
            </button>
            <button
              className={styles.btnPrimary}
              onClick={handlePublish}
              disabled={publishing}
            >
              {publishing ? '发布中\u2026' : '发布活动'}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
