'use client';

import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';

import {
    getSession,
    getSystemConfig,
    type Interaction,
    type StudentSession,
    type WSEvent,
} from '@/lib/api';
import { usePluginRegistry } from '@/lib/plugin/PluginRegistry';
import { useBuiltinSkillRenderers } from '@/lib/plugin/SkillRendererPlugins';
import { getMissingRendererSkillIds } from '@/lib/plugin/SkillManifestLoader';
import type { SkillRendererProps, InteractionEvent } from '@/lib/plugin/types';
import {
    getCachedResponse,
    setCachedResponse,
    purgeExpiredEntries,
} from '@/lib/cache/indexedDBCache';
import { useToast } from '@/components/Toast';
import LoadingSpinner from '@/components/LoadingSpinner';
import { useSessionWebSocket, type WSStatus } from './hooks/useSessionWebSocket';
import MessageList, { type ChatMessage } from './components/MessageList';
import ScaffoldPanel, {
    type ScaffoldLevel,
    type ScaffoldData,
    SCAFFOLD_LABELS,
    SCAFFOLD_DESCRIPTIONS,
} from './components/ScaffoldPanel';
import SessionInput from './components/SessionInput';
import styles from './page.module.css';



// -- Connection Status Label -------------------------------------

const WS_STATUS_LABELS: Record<WSStatus, string> = {
    connecting: '连接中...',
    connected: '已连接',
    reconnecting: '重连中...',
    disconnected: '连接断开',
};

// -- Component ---------------------------------------------------

export default function SessionPage() {
    const params = useParams();
    const router = useRouter();
    const { toast } = useToast();
    const missingToastShownRef = useRef(false);
    const sessionId = Number(params.id);

    // Register built-in skill renderers with plugin system
    useBuiltinSkillRenderers();

    useEffect(() => {
        if (missingToastShownRef.current) return;
        const missing = getMissingRendererSkillIds();
        if (missing.length > 0) {
            toast(`缺少技能渲染器: ${missing.join(', ')}`, 'warning');
            missingToastShownRef.current = true;
        }
    }, [toast]);

    // Core state
    const [session, setSession] = useState<StudentSession | null>(null);
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [loading, setLoading] = useState(true);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);

    // Override state
    const [providerOverride, setProviderOverride] = useState('');
    const [modelOverride, setModelOverride] = useState('');

    // Scaffold state
    const [scaffoldLevel, setScaffoldLevel] = useState<ScaffoldLevel>('high');
    const [scaffoldData] = useState<ScaffoldData>({});
    const [scaffoldTransition, setScaffoldTransition] = useState(false);

    // Agent thinking state (T-4.14)
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);

    // Streaming token buffer
    const [streamingContent, setStreamingContent] = useState('');
    const lastResponseRef = useRef('');

    // L1 cache: track pending question for caching the response
    const pendingQuestionRef = useRef<string | null>(null);

    // -- Plugin System: find matching skill renderer ----------------

    const plugins = usePluginRegistry('student.interaction.main');
    const activeSkill = session?.active_skill || '';
    const matchedPlugin = useMemo(() => {
        if (!activeSkill) return null;
        return plugins.find(p => p.id === `skill-renderer-${activeSkill}`) || null;
    }, [plugins, activeSkill]);
    const activePlugin = matchedPlugin?.Component ? matchedPlugin : null;

    // -- WebSocket Event Handler ------------------------------------

    const handleWSEvent = useCallback((event: WSEvent) => {
        // Skip events when a plugin renderer handles the WebSocket directly
        if (activePlugin) return;

        switch (event.event) {
            case 'agent_thinking': {
                const payload = event.payload as { status: string };
                setThinkingStatus(payload.status);
                break;
            }

            case 'token_delta': {
                const payload = event.payload as { text?: string; content?: string; delta?: string };
                const delta = payload.text ?? payload.content ?? payload.delta ?? '';
                if (!delta) break;
                setThinkingStatus(null);
                setStreamingContent(prev => {
                    const next = prev + delta;
                    lastResponseRef.current = next;
                    return next;
                });
                break;
            }

            case 'ui_scaffold_change': {
                const payload = event.payload as {
                    action: string;
                    data: {
                        old_level: string;
                        new_level: ScaffoldLevel;
                        mastery: number;
                        kp_id: number;
                        direction: string;
                    };
                };

                setScaffoldTransition(true);
                setScaffoldLevel(payload.data.new_level);

                const direction = payload.data.direction === 'fade' ? '降低' : '增强';
                const newLabel = SCAFFOLD_LABELS[payload.data.new_level];
                setMessages(prev => [
                    ...prev,
                    {
                        id: `sys-${Date.now()}`,
                        role: 'system',
                        content: `支架已${direction}至 ${newLabel} (掌握度: ${(payload.data.mastery * 100).toFixed(0)}%)`,
                        timestamp: Date.now(),
                    },
                ]);

                setTimeout(() => setScaffoldTransition(false), 500);
                break;
            }


            case 'system_message': {
                const payload = event.payload as { content: string };
                setMessages(prev => [
                    ...prev,
                    {
                        id: `sys-${Date.now()}`,
                        role: 'system',
                        content: payload.content,
                        timestamp: Date.now(),
                    }
                ]);
                break;
            }

            case 'teacher_takeover': {
                const payload = event.payload as { id: number, content: string, created_at: string };
                setMessages(prev => [
                    ...prev,
                    {
                        id: `t-${payload.id || Date.now()}`,
                        role: 'teacher',
                        content: payload.content,
                        timestamp: payload.created_at ? new Date(payload.created_at).getTime() : Date.now(),
                    }
                ]);
                // Clear any ongoing AI thinking/streaming since teacher interrupted
                setThinkingStatus(null);
                setStreamingContent('');
                break;
            }

            case 'turn_complete': {
                console.log('[SESSION DEBUG] ✅ turn_complete 收到, 流式内容长度:', streamingContent.length);
                setThinkingStatus(null);
                setSending(false);

                setStreamingContent(prev => {
                    const content = prev || lastResponseRef.current;
                    if (content) {
                        setMessages(msgs => [
                            ...msgs,
                            {
                                id: `coach-${Date.now()}`,
                                role: 'coach',
                                content,
                                timestamp: Date.now(),
                            },
                        ]);

                        const pendingQ = pendingQuestionRef.current;
                        if (pendingQ) {
                            setCachedResponse(sessionId, pendingQ, content);
                            pendingQuestionRef.current = null;
                        }
                    }
                    lastResponseRef.current = '';
                    return '';
                });
                break;
            }

            case 'error': {
                const payload = event.payload as { message: string };
                console.error('[SESSION DEBUG] ❌ 收到服务端错误:', payload.message);
                setThinkingStatus(null);
                setSending(false);
                lastResponseRef.current = '';
                toast(payload.message, 'error');
                break;
            }
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [toast, matchedPlugin]);

    // -- WebSocket Hook ---------------------------------------------

    const { wsStatus, reconnectCount, agentChannel } = useSessionWebSocket({
        sessionId,
        sessionStatus: session?.status,
        onEvent: handleWSEvent,
    });

    // -- Interaction event handler for analytics --------------------

    const handleInteractionEvent = useCallback((event: InteractionEvent) => {
        console.log('[Plugin] Interaction event:', event.type, event.payload);
    }, []);

    // -- L1 Cache: purge expired entries on mount -------------------

    useEffect(() => {
        purgeExpiredEntries().then(purged => {
            if (purged > 0) {
                console.log(`[L1 Cache] Purged ${purged} expired entries`);
            }
        });
    }, []);

    // -- Load system AI config for debugging -------------------------

    useEffect(() => {
        getSystemConfig()
            .then((config) => {
                const provider = config['LLM_PROVIDER'] || '(未配置)';
                const dashscopeModel = config['DASHSCOPE_MODEL'] || '(默认 qwen-max)';
                const dashscopeBaseUrl = config['DASHSCOPE_COMPAT_BASE_URL'] || 'https://dashscope.aliyuncs.com/compatible-mode/v1';
                const ollamaModel = config['OLLAMA_MODEL'] || '(默认 qwen2.5:7b)';
                const ollamaBaseUrl = config['OLLAMA_BASE_URL'] || 'http://localhost:11434';
                const embeddingModel = config['EMBEDDING_MODEL'] || '(默认)';
                const embeddingProvider = config['EMBEDDING_PROVIDER'] || provider;

                console.log(
                    '%c[AI CONFIG DEBUG] 系统默认 AI 配置信息',
                    'background: #1a73e8; color: white; padding: 4px 8px; border-radius: 4px; font-weight: bold;'
                );
                console.table({
                    '当前 Provider': { value: provider },
                    'DashScope Model': { value: dashscopeModel },
                    'DashScope Base URL': { value: dashscopeBaseUrl },
                    'DashScope Chat URL': { value: dashscopeBaseUrl.replace(/\/$/, '') + '/chat/completions' },
                    'Ollama Model': { value: ollamaModel },
                    'Ollama Base URL': { value: ollamaBaseUrl },
                    'Embedding Provider': { value: embeddingProvider },
                    'Embedding Model': { value: embeddingModel },
                });
                console.log('[AI CONFIG DEBUG] 完整配置:', config);
            })
            .catch((err) => {
                console.warn('[AI CONFIG DEBUG] 获取系统配置失败:', err);
            });
    }, []);

    // -- Load session data ------------------------------------------

    useEffect(() => {
        if (!sessionId) return;

        const loadSession = async () => {
            try {
                const data = await getSession(sessionId);
                setSession(data.session);
                setScaffoldLevel(data.session.scaffold_level || 'high');

                const existingMessages: ChatMessage[] = data.interactions.map(
                    (interaction: Interaction) => ({
                        id: `i-${interaction.id}`,
                        role: interaction.role as ChatMessage['role'],
                        content: interaction.content,
                        timestamp: new Date(interaction.created_at).getTime(),
                    })
                );
                setMessages(existingMessages);
            } catch (err) {
                console.error('Failed to load session:', err);
                toast('加载会话失败', 'error');
                router.push('/student/activities');
            } finally {
                setLoading(false);
            }
        };

        loadSession();
    }, [sessionId, router, toast]);

    // -- Send Message -----------------------------------------------

    const handleSend = useCallback(async () => {
        const text = input.trim();
        if (!text || sending) return;
        if (wsStatus !== 'connected') {
            toast('连接未就绪，请稍后重试', 'warning');
            return;
        }

        setMessages(prev => [
            ...prev,
            {
                id: `student-${Date.now()}`,
                role: 'student',
                content: text,
                timestamp: Date.now(),
            },
        ]);

        setInput('');

        // L1 Cache: check for cached response before sending
        const cached = await getCachedResponse(sessionId, text);
        if (cached) {
            console.log('[L1 Cache] Hit — returning cached response');
            setMessages(prev => [
                ...prev,
                {
                    id: `coach-cache-${Date.now()}`,
                    role: 'coach',
                    content: cached,
                    timestamp: Date.now(),
                },
            ]);
            return;
        }

        // Cache miss — send via WebSocket
        pendingQuestionRef.current = text;
        setSending(true);
        setStreamingContent('');
        lastResponseRef.current = '';


        const payload: Record<string, string> = { text };
        if (providerOverride) {
            payload.provider_override = providerOverride;
        }
        if (modelOverride) {
            payload.model_override = modelOverride;
        }

        const event: WSEvent = {
            event: 'user_message',
            payload,
            timestamp: Math.floor(Date.now() / 1000),
        };

        console.log('[SESSION DEBUG] 发送消息详情:', {
            text: text.substring(0, 100),
            providerOverride: providerOverride || '(未设置, 使用系统默认)',
            modelOverride: modelOverride || '(未设置, 使用系统默认)',
            wsStatus,
            fullPayload: JSON.stringify(event),
        });

        agentChannel.send(JSON.stringify(event));
    }, [input, sending, sessionId, wsStatus, agentChannel, providerOverride, modelOverride, toast]); // eslint-disable-line react-hooks/exhaustive-deps

    // -- Render Skill Renderer (plugin mode) ------------------------

    const renderSkillRenderer = () => {
        if (!activePlugin || !session) return null;
        const RendererComponent = activePlugin.Component as unknown as React.FC<SkillRendererProps>;
        const rendererProps: SkillRendererProps = {
            studentContext: {
                studentId: session.student_id,
                displayName: '',
                courseId: 0,
                sessionId: session.id,
            },
            knowledgePoint: {
                id: session.current_kp_id,
                title: '',
                difficulty: 0,
                chapterTitle: '',
            },
            scaffoldingLevel: scaffoldLevel,
            agentChannel,
            onInteractionEvent: handleInteractionEvent,
        };
        return <RendererComponent {...rendererProps} />;
    };

    // -- Loading State ----------------------------------------------

    if (loading) {
        return <LoadingSpinner size="large" />;
    }

    if (!session) {
        return (
            <div style={{ textAlign: 'center', padding: '80px 0', color: 'var(--text-muted)' }}>
                会话不存在
            </div>
        );
    }

    // -- Main Render ------------------------------------------------

    return (
        <div className="fade-in">
            <div className={styles.chatContainer}>
                {/* Sandbox Banner */}
                {session.is_sandbox && (
                    <div className={styles.sandboxBanner}>
                        <span className={styles.sandboxBannerIcon}>🔬</span>
                        沙盒预览模式 — 当前为教师预览视角，学习数据不会被记录
                    </div>
                )}

                {/* Header */}
                <div className={styles.chatHeader}>
                    <div>
                        <Link
                            href={session.is_sandbox ? '/teacher/dashboard' : '/student/activities'}
                            className={styles.backLink}
                        >
                            &larr; {session.is_sandbox ? '返回教师仪表盘' : '返回活动列表'}
                        </Link>
                        <div className={styles.chatTitle}>AI 学习对话</div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                        {/* Connection Status */}
                        {wsStatus !== 'connected' && (
                            <div className={`${styles.connectionStatus} ${styles[`ws_${wsStatus}`]}`} role="status" aria-live="polite">
                                <span className={styles.connectionDot} aria-hidden="true" />
                                {wsStatus === 'reconnecting'
                                    ? `重连中 (${reconnectCount}/8)...`
                                    : WS_STATUS_LABELS[wsStatus]}
                            </div>
                        )}
                        <div className={`${styles.scaffoldBadge} ${scaffoldLevel === 'high' ? styles.scaffoldHigh :
                            scaffoldLevel === 'medium' ? styles.scaffoldMedium :
                                styles.scaffoldLow
                            }`}>
                            {SCAFFOLD_LABELS[scaffoldLevel]}
                            <span style={{ fontSize: '10px', opacity: 0.7 }}>
                                &middot; {SCAFFOLD_DESCRIPTIONS[scaffoldLevel]}
                            </span>
                        </div>
                    </div>
                </div>

                {/* Skill Renderer or Default Chat */}
                {activePlugin ? renderSkillRenderer() : (
                    <>
                        <MessageList
                            messages={messages}
                            streamingContent={streamingContent}
                            thinkingStatus={thinkingStatus}
                        />
                        <ScaffoldPanel
                            level={scaffoldLevel}
                            data={scaffoldData}
                            transition={scaffoldTransition}
                        />
                        <SessionInput
                            input={input}
                            providerOverride={providerOverride}
                            setProviderOverride={setProviderOverride}
                            modelOverride={modelOverride}
                            setModelOverride={setModelOverride}
                            setInput={setInput}
                            sending={sending}
                            sessionActive={session.status === 'active'}
                            onSend={handleSend}
                            agentChannel={agentChannel}
                        />
                    </>
                )}


            </div>
        </div>
    );
}
