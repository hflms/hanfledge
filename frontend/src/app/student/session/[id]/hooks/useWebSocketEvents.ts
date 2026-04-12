import { useCallback } from 'react';
import type { WSEvent, StudentSession } from '@/lib/api';
import type { ScaffoldLevel } from '../components/ScaffoldPanel';
import { generateId } from '@/lib/utils';
import { SCAFFOLD_LABELS } from '../components/ScaffoldPanel';
import type { ChatMessage } from '../components/MessageList';

interface UseWebSocketEventsProps {
    activePlugin: unknown;
    setThinkingStatus: (status: string | null) => void;
    handleStreamingDelta: (delta: string) => void;
    handleStreamingComplete: () => void;
    setSession: React.Dispatch<React.SetStateAction<StudentSession | null>>;
    setScaffoldTransition: (val: boolean) => void;
    setScaffoldLevel: (val: ScaffoldLevel) => void;
    addMessage: (msg: ChatMessage) => void;
    setSending: (val: boolean) => void;
    toast: (msg: string, type?: 'success' | 'error' | 'warning' | 'info') => void;
    setStreamingContent: (val: string) => void;
}

export function useWebSocketEvents({
    activePlugin,
    setThinkingStatus,
    handleStreamingDelta,
    handleStreamingComplete,
    setSession,
    setScaffoldTransition,
    setScaffoldLevel,
    addMessage,
    setSending,
    toast,
    setStreamingContent
}: UseWebSocketEventsProps) {
    const handleWSEvent = useCallback((event: WSEvent) => {
        switch (event.event) {
            case 'agent_thinking':
            case 'token_delta':
            case 'turn_complete': {
                if (activePlugin) return; // Skill renderer 会处理这些事件

                if (event.event === 'agent_thinking') {
                    const payload = event.payload as { status: string };
                    setThinkingStatus(payload.status);
                } else if (event.event === 'token_delta') {
                    const payload = event.payload as { text?: string; content?: string; delta?: string };
                    const delta = payload.text ?? payload.content ?? payload.delta ?? '';
                    if (!delta) break;
                    handleStreamingDelta(delta);
                } else if (event.event === 'turn_complete') {
                    console.log('[SESSION DEBUG] ✅ turn_complete 收到');
                    handleStreamingComplete();
                }
                break;
            }

            case 'ui_scaffold_change': {
                const payload = event.payload as {
                    action: string;
                    data: {
                        new_skill?: string;
                        new_level?: string;
                        mastery?: number;
                        direction?: string;
                    };
                };

                if (payload.action === 'skill_change') {
                    const newSkill = payload.data.new_skill as string;
                    setSession(prev => prev ? { ...prev, active_skill: newSkill } : null);
                    toast(`系统已根据您的掌握度自动切换到技能: ${newSkill}`, 'success');
                    break;
                }

                setScaffoldTransition(true);
                const newLevel = payload.data.new_level as ScaffoldLevel;
                setScaffoldLevel(newLevel);

                const direction = payload.data.direction === 'fade' ? '降低' : '增强';
                const newLabel = SCAFFOLD_LABELS[newLevel];
                const mastery = payload.data.mastery ?? 0;

                addMessage({
                    id: generateId('sys'),
                    role: 'system',
                    content: `支架已${direction}至 ${newLabel} (掌握度: ${(mastery * 100).toFixed(0)}%)`,
                    timestamp: Date.now(),
                });

                setTimeout(() => setScaffoldTransition(false), 500);
                break;
            }

            case 'system_message': {
                const payload = event.payload as { content: string };
                addMessage({
                    id: generateId('sys'),
                    role: 'system',
                    content: payload.content,
                    timestamp: Date.now(),
                });
                break;
            }

            case 'teacher_takeover': {
                const payload = event.payload as { id: number, content: string, created_at: string };
                addMessage({
                    id: generateId(`t-${payload.id || Date.now()}`),
                    role: 'teacher',
                    content: payload.content,
                    timestamp: payload.created_at ? new Date(payload.created_at).getTime() : Date.now(),
                });
                setThinkingStatus(null);
                setStreamingContent('');
                break;
            }

            case 'error': {
                const payload = event.payload as { message: string };
                console.error('[SESSION DEBUG] ❌ 收到服务端错误:', payload.message);
                setThinkingStatus(null);
                setSending(false);
                setStreamingContent(''); // Reset on error
                toast(payload.message, 'error');
                break;
            }
        }
    }, [
        activePlugin,
        setThinkingStatus,
        handleStreamingDelta,
        handleStreamingComplete,
        setSession,
        setScaffoldTransition,
        setScaffoldLevel,
        addMessage,
        setSending,
        toast,
        setStreamingContent
    ]);

    return { handleWSEvent };
}
