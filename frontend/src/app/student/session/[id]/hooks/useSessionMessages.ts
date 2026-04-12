import { useState, useRef, useEffect, useCallback } from 'react';
import { getSession, type Interaction, type StudentSession, type LearningActivity } from '@/lib/api';
import { getCachedResponse, setCachedResponse, purgeExpiredEntries } from '@/lib/cache/indexedDBCache';
import { generateId } from '@/lib/utils';
import type { ChatMessage } from '../components/MessageList';

interface UseSessionMessagesProps {
    sessionId: number;
    onError: (msg: string) => void;
    onSessionLoaded?: (session: StudentSession, activity: LearningActivity) => void;
}

export function useSessionMessages({ sessionId, onError, onSessionLoaded }: UseSessionMessagesProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [streamingContent, setStreamingContent] = useState('');
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
    const [sending, setSending] = useState(false);

    const lastResponseRef = useRef('');
    const pendingQuestionRef = useRef<string | null>(null);
    const [session, setSession] = useState<StudentSession | null>(null);
    const [activity, setActivity] = useState<LearningActivity | null>(null);
    const [loading, setLoading] = useState(true);

    const autoStartTriggeredRef = useRef(false);

    useEffect(() => {
        purgeExpiredEntries().then(purged => {
            if (purged > 0) {
                console.log(`[L1 Cache] Purged ${purged} expired entries`);
            }
        });
    }, []);

    const loadSessionData = useCallback(async () => {
        if (!sessionId) return;

        try {
            const data = await getSession(sessionId);
            setSession(data.session);
            setActivity(data.activity);

            const existingMessages: ChatMessage[] = data.interactions.map(
                (interaction: Interaction) => ({
                    id: `i-${interaction.id}`,
                    role: interaction.role as ChatMessage['role'],
                    content: interaction.content,
                    timestamp: new Date(interaction.created_at).getTime(),
                })
            );
            setMessages(existingMessages);

            if (existingMessages.length === 0 && data.session.status === 'active' && !autoStartTriggeredRef.current) {
                autoStartTriggeredRef.current = true;
                console.log('[SESSION] 新会话,准备自动开始学习活动');
            }

            if (onSessionLoaded) {
                onSessionLoaded(data.session, data.activity);
            }
        } catch (err) {
            console.error('Failed to load session:', err);
            onError('加载会话失败');
        } finally {
            setLoading(false);
        }
    }, [sessionId, onError, onSessionLoaded]);

    useEffect(() => {
        loadSessionData();
    }, [loadSessionData]);

    const addMessage = useCallback((msg: ChatMessage) => {
        setMessages(prev => [...prev, msg]);
    }, []);

    const handleStreamingDelta = useCallback((delta: string) => {
        setThinkingStatus(null);
        setStreamingContent(prev => {
            const next = prev + delta;
            lastResponseRef.current = next;
            return next;
        });
    }, []);

    const handleStreamingComplete = useCallback(() => {
        setThinkingStatus(null);
        setSending(false);

        setStreamingContent(prev => {
            const content = prev || lastResponseRef.current;
            if (content) {
                addMessage({
                    id: generateId('coach'),
                    role: 'coach',
                    content,
                    timestamp: Date.now(),
                });

                const pendingQ = pendingQuestionRef.current;
                if (pendingQ) {
                    setCachedResponse(sessionId, pendingQ, content);
                    pendingQuestionRef.current = null;
                }
            }
            lastResponseRef.current = '';
            return '';
        });
    }, [sessionId, addMessage]);

    const setPendingQuestion = useCallback((q: string) => {
        pendingQuestionRef.current = q;
    }, []);

    const clearPendingQuestion = useCallback(() => {
        pendingQuestionRef.current = null;
    }, []);

    return {
        messages,
        setMessages,
        addMessage,
        streamingContent,
        setStreamingContent,
        thinkingStatus,
        setThinkingStatus,
        sending,
        setSending,
        handleStreamingDelta,
        handleStreamingComplete,
        setPendingQuestion,
        clearPendingQuestion,
        session,
        setSession,
        activity,
        loading,
        autoStartTriggeredRef
    };
}
