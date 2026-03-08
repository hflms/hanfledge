/**
 * useSkillHistory - Hook for managing skill-generated content history
 * 
 * Provides a unified way to track and retrieve skill outputs (presentations,
 * quizzes, surveys, etc.) that can be reopened from a history drawer.
 */

import { useState, useCallback } from 'react';
import type { SkillHistoryItem } from '@/components/SkillHistoryDrawer';

export interface SkillHistoryEntry<T = unknown> {
    id: string;
    type: 'presentation' | 'quiz' | 'survey' | 'other';
    title: string;
    timestamp: number;
    icon: string;
    data: T;
}

export function useSkillHistory<T = unknown>() {
    const [entries, setEntries] = useState<SkillHistoryEntry<T>[]>([]);

    const addEntry = useCallback((
        type: SkillHistoryEntry<T>['type'],
        title: string,
        data: T,
        icon: string = '📄'
    ): string => {
        const id = `${type}-${Date.now()}`;
        const entry: SkillHistoryEntry<T> = {
            id,
            type,
            title,
            timestamp: Date.now(),
            icon,
            data,
        };
        setEntries(prev => [...prev, entry]);
        return id;
    }, []);

    const getEntry = useCallback((id: string): SkillHistoryEntry<T> | null => {
        return entries.find(e => e.id === id) || null;
    }, [entries]);

    const clearHistory = useCallback(() => {
        setEntries([]);
    }, []);

    // Convert to SkillHistoryItem format for drawer
    const historyItems: SkillHistoryItem[] = entries.map(e => ({
        id: e.id,
        type: e.type,
        title: e.title,
        timestamp: e.timestamp,
        icon: e.icon,
    }));

    return {
        entries,
        historyItems,
        addEntry,
        getEntry,
        clearHistory,
    };
}
