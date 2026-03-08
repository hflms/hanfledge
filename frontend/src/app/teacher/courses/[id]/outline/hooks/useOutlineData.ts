/**
 * Hook for outline data fetching
 */

import { useState, useCallback, useEffect } from 'react';
import { getCourseOutline, getDocuments, listClasses, listTeacherActivities, type Course, type Document, type ClassItem, type LearningActivity } from '@/lib/api';

export function useOutlineData(courseId: number, onError: (msg: string, redirect?: boolean) => void) {
  const [course, setCourse] = useState<Course | null>(null);
  const [docs, setDocs] = useState<Document[]>([]);
  const [classes, setClasses] = useState<ClassItem[]>([]);
  const [activities, setActivities] = useState<LearningActivity[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchData = useCallback(async () => {
    if (!courseId || isNaN(courseId)) {
      onError('课程不存在，已返回课程列表', true);
      return;
    }
    try {
      const data = await getCourseOutline(courseId);
      setCourse(data.course);
      setDocs(data.documents || []);
      const activityRes = await listTeacherActivities(courseId, { page: 1, limit: 50 });
      setActivities(activityRes.items || []);
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : String(err);
      if (errorMsg.includes('课程不存在')) {
        onError('课程不存在，已返回课程列表', true);
      } else {
        onError('获取课程大纲失败');
      }
    } finally {
      setLoading(false);
    }
  }, [courseId, onError]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  useEffect(() => {
    const loadClasses = async () => {
      try {
        const res = await listClasses({ page: 1, limit: 100 });
        setClasses(res.items || []);
      } catch (err) {
        console.warn('Failed to load classes', err);
      }
    };
    loadClasses();
  }, []);

  return {
    course,
    docs,
    classes,
    activities,
    loading,
    refetch: fetchData,
  };
}
