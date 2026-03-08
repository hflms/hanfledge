/**
 * Hook for AI skill recommendation
 */

import { useState, useCallback } from 'react';
import { recommendSkills, batchMountSkills, type RecommendMount } from '@/lib/api';

export function useSkillRecommendation(courseId: number, onSuccess: () => void, onError: (msg: string) => void) {
  const [recommending, setRecommending] = useState(false);
  const [recommendations, setRecommendations] = useState<RecommendMount[] | null>(null);
  const [selectedMounts, setSelectedMounts] = useState<Set<string>>(new Set());
  const [batchMounting, setBatchMounting] = useState(false);

  const fetchRecommendations = useCallback(async () => {
    setRecommending(true);
    try {
      const res = await recommendSkills(courseId);
      setRecommendations(res.recommendations || []);
      const allSet = new Set<string>();
      (res.recommendations || []).forEach((_, i) => allSet.add(i.toString()));
      setSelectedMounts(allSet);
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setRecommending(false);
    }
  }, [courseId, onError]);

  const applyRecommendations = useCallback(async () => {
    if (!recommendations) return;
    const mountsToApply = recommendations
      .filter((_, i) => selectedMounts.has(i.toString()))
      .map(r => ({ kp_id: r.kp_id, skill_id: r.skill_id, scaffold_level: 'high' }));
    
    if (mountsToApply.length === 0) return;

    setBatchMounting(true);
    try {
      await batchMountSkills(courseId, mountsToApply);
      onSuccess();
      setRecommendations(null);
      setSelectedMounts(new Set());
    } catch (err) {
      onError(err instanceof Error ? err.message : String(err));
    } finally {
      setBatchMounting(false);
    }
  }, [courseId, recommendations, selectedMounts, onSuccess, onError]);

  const toggleMount = useCallback((index: number) => {
    setSelectedMounts(prev => {
      const next = new Set(prev);
      const key = index.toString();
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  }, []);

  const closeRecommendations = useCallback(() => {
    setRecommendations(null);
    setSelectedMounts(new Set());
  }, []);

  return {
    recommending,
    recommendations,
    selectedMounts,
    batchMounting,
    fetchRecommendations,
    applyRecommendations,
    toggleMount,
    closeRecommendations,
  };
}
