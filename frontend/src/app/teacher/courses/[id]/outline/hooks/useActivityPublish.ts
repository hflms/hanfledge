/**
 * Hook for activity publishing
 */

import { useState, useCallback } from 'react';
import { createActivity, publishActivity, type ClassItem, type LearningActivity } from '@/lib/api';

export function useActivityPublish(courseId: number, onSuccess: () => void, onError: (msg: string) => void) {
  const [activityTitle, setActivityTitle] = useState('');
  const [activityDeadline, setActivityDeadline] = useState('');
  const [activityAllowRetry, setActivityAllowRetry] = useState(true);
  const [activityMaxAttempts, setActivityMaxAttempts] = useState(3);
  const [selectedKP, setSelectedKP] = useState<Set<number>>(new Set());
  const [selectedClasses, setSelectedClasses] = useState<Set<number>>(new Set());
  const [creatingActivity, setCreatingActivity] = useState(false);
  const [publishingId, setPublishingId] = useState<number | null>(null);

  const toggleKP = useCallback((kpId: number) => {
    setSelectedKP(prev => {
      const next = new Set(prev);
      if (next.has(kpId)) {
        next.delete(kpId);
      } else {
        next.add(kpId);
      }
      return next;
    });
  }, []);

  const toggleClass = useCallback((classId: number) => {
    setSelectedClasses(prev => {
      const next = new Set(prev);
      if (next.has(classId)) {
        next.delete(classId);
      } else {
        next.add(classId);
      }
      return next;
    });
  }, []);

  const createNewActivity = useCallback(async () => {
    if (!activityTitle.trim()) {
      onError('请输入活动标题');
      return;
    }
    if (selectedKP.size === 0) {
      onError('请至少选择一个知识点');
      return;
    }

    setCreatingActivity(true);
    try {
      await createActivity({
        course_id: courseId,
        title: activityTitle.trim(),
        kp_ids: Array.from(selectedKP),
        class_ids: selectedClasses.size > 0 ? Array.from(selectedClasses) : undefined,
        deadline: activityDeadline || undefined,
        allow_retry: activityAllowRetry,
        max_attempts: activityMaxAttempts,
      });
      onSuccess();
      setActivityTitle('');
      setActivityDeadline('');
      setSelectedKP(new Set());
      setSelectedClasses(new Set());
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setCreatingActivity(false);
    }
  }, [courseId, activityTitle, activityDeadline, activityAllowRetry, activityMaxAttempts, selectedKP, selectedClasses, onSuccess, onError]);

  const publishExisting = useCallback(async (activityId: number) => {
    setPublishingId(activityId);
    try {
      await publishActivity(activityId);
      onSuccess();
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setPublishingId(null);
    }
  }, [onSuccess, onError]);

  return {
    activityTitle,
    activityDeadline,
    activityAllowRetry,
    activityMaxAttempts,
    selectedKP,
    selectedClasses,
    creatingActivity,
    publishingId,
    setActivityTitle,
    setActivityDeadline,
    setActivityAllowRetry,
    setActivityMaxAttempts,
    toggleKP,
    toggleClass,
    createNewActivity,
    publishExisting,
  };
}
