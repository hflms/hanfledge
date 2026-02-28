'use client';

import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';
import {
    getSession,
    createWSUrl,
    type Interaction,
    type StudentSession,
    type WSEvent,
} from '@/lib/api';
import MarkdownRenderer from '@/components/MarkdownRenderer';
import { usePluginRegistry } from '@/lib/plugin/PluginRegistry';
import { useBuiltinSkillRenderers } from '@/lib/plugin/SkillRendererPlugins';
import type { AgentWebSocketChannel, SkillRendererProps, InteractionEvent } from '@/lib/plugin/types';
import styles from './page.module.css';

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
}

type ScaffoldLevel = 'high' | 'medium' | 'low';

type WSStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

// -- Reconnect Constants -----------------------------------------

const WS_RECONNECT_BASE_DELAY = 1000;  // 1s initial delay
const WS_RECONNECT_MAX_DELAY = 30000;  // 30s max delay
const WS_RECONNECT_MAX_RETRIES = 8;
const WS_PING_INTERVAL = 30000;        // 30s heartbeat ping

interface ScaffoldData {
    steps?: string[];
    keywords?: string[];
}

// -- Scaffold Labels ---------------------------------------------

const SCAFFOLD_LABELS: Record<ScaffoldLevel, string> = {
    high: '高支架',
    medium: '中支架',
    low: '低支架',
};

const SCAFFOLD_DESCRIPTIONS: Record<ScaffoldLevel, string> = {
    high: '分步引导模式 — 按照步骤思考',
    medium: '关键词提示模式 — 围绕关键概念思考',
    low: '自由思考模式 — 独立解决问题',
};

// -- Component ---------------------------------------------------

export default function SessionPage() {
    const params = useParams();
    const router = useRouter();
    const sessionId = Number(params.id);

    // Register built-in skill renderers with plugin system
    useBuiltinSkillRenderers();

    // Core state
    const [session, setSession] = useState<StudentSession | null>(null);
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [loading, setLoading] = useState(true);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);

    // Scaffold state
    const [scaffoldLevel, setScaffoldLevel] = useState<ScaffoldLevel>('high');
    const [scaffoldData] = useState<ScaffoldData>({});
    const [scaffoldTransition, setScaffoldTransition] = useState(false);

    // Agent thinking state (T-4.14)
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);

    // Streaming token buffer
    const [streamingContent, setStreamingContent] = useState('');

    // WebSocket connection status
    const [wsStatus, setWsStatus] = useState<WSStatus>('disconnected');

    // WebSocket ref
    const wsRef = useRef<WebSocket | null>(null);
    const messagesEndRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLTextAreaElement>(null);
    const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
    const reconnectCountRef = useRef(0);
    const intentionalCloseRef = useRef(false);

    // -- Plugin System: find matching skill renderer -------------

    const plugins = usePluginRegistry('student.interaction.main');
    const activeSkill = session?.active_skill || '';
    const matchedPlugin = useMemo(() => {
        if (!activeSkill) return null;
        return plugins.find(p => p.id === `skill-renderer-${activeSkill}`) || null;
    }, [plugins, activeSkill]);

    // -- AgentWebSocketChannel adapter ---------------------------

    // Store WS message/close handlers as refs so renderers can subscribe
    const wsMessageHandlersRef = useRef<Array<(data: string) => void>>([]);
    const wsCloseHandlersRef = useRef<Array<() => void>>([]);

    const agentChannel = useMemo<AgentWebSocketChannel>(() => ({
        send: (message: string) => {
            if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
                wsRef.current.send(message);
            }
        },
        onMessage: (handler: (data: string) => void) => {
            wsMessageHandlersRef.current.push(handler);
        },
        onClose: (handler: () => void) => {
            wsCloseHandlersRef.current.push(handler);
        },
        close: () => {
            intentionalCloseRef.current = true;
            wsRef.current?.close();
        },
    }), []);

    // -- Interaction event handler for analytics -----------------

    const handleInteractionEvent = useCallback((event: InteractionEvent) => {
        console.log('[Plugin] Interaction event:', event.type, event.payload);
        // Future: send to analytics endpoint
    }, []);

    // -- Scroll to bottom ----------------------------------------

    const scrollToBottom = useCallback(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, []);

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingContent, thinkingStatus, scrollToBottom]);

    // -- Load session data ---------------------------------------

    useEffect(() => {
        if (!sessionId) return;

        const loadSession = async () => {
            try {
                const data = await getSession(sessionId);
                setSession(data.session);
                setScaffoldLevel(data.session.scaffold_level || 'high');

                // Convert existing interactions to chat messages
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
                alert('加载会话失败');
                router.push('/student/activities');
            } finally {
                setLoading(false);
            }
        };

        loadSession();
    }, [sessionId, router]);

    // -- WebSocket connection ------------------------------------

    const connectWebSocket = useCallback(() => {
        if (!session || session.status !== 'active') return;

        const wsUrl = createWSUrl(sessionId);
        setWsStatus(reconnectCountRef.current > 0 ? 'reconnecting' : 'connecting');

        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            console.log('[WS] Connected to session', sessionId);
            setWsStatus('connected');
            reconnectCountRef.current = 0;

            // Start heartbeat ping interval
            pingTimerRef.current = setInterval(() => {
                if (ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ event: 'ping', payload: {}, timestamp: Math.floor(Date.now() / 1000) }));
                }
            }, WS_PING_INTERVAL);
        };

        ws.onmessage = (event) => {
            try {
                const wsEvent: WSEvent = JSON.parse(event.data);

                // Forward raw message to all plugin-registered handlers
                for (const handler of wsMessageHandlersRef.current) {
                    handler(event.data);
                }

                // Also handle in the default chat UI (for fallback mode)
                if (!matchedPlugin) {
                    handleWSEvent(wsEvent);
                }
            } catch (err) {
                console.error('[WS] Parse error:', err);
            }
        };

        ws.onclose = (event) => {
            console.log('[WS] Disconnected:', event.code, event.reason);
            wsRef.current = null;

            // Notify plugin close handlers
            for (const handler of wsCloseHandlersRef.current) {
                handler();
            }

            // Stop heartbeat
            if (pingTimerRef.current) {
                clearInterval(pingTimerRef.current);
                pingTimerRef.current = null;
            }

            // Auto-reconnect unless intentionally closed
            if (!intentionalCloseRef.current && reconnectCountRef.current < WS_RECONNECT_MAX_RETRIES) {
                const delay = Math.min(
                    WS_RECONNECT_BASE_DELAY * Math.pow(2, reconnectCountRef.current),
                    WS_RECONNECT_MAX_DELAY
                );
                reconnectCountRef.current += 1;
                setWsStatus('reconnecting');
                console.log(`[WS] Reconnecting in ${delay}ms (attempt ${reconnectCountRef.current}/${WS_RECONNECT_MAX_RETRIES})`);
                reconnectTimerRef.current = setTimeout(connectWebSocket, delay);
            } else {
                setWsStatus('disconnected');
                if (reconnectCountRef.current >= WS_RECONNECT_MAX_RETRIES) {
                    console.warn('[WS] Max reconnect attempts reached');
                }
            }
        };

        ws.onerror = (err) => {
            console.error('[WS] Error:', err);
        };
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [session, sessionId, matchedPlugin]);

    useEffect(() => {
        if (!session || session.status !== 'active') return;

        intentionalCloseRef.current = false;
        // Clear plugin handlers on reconnect
        wsMessageHandlersRef.current = [];
        wsCloseHandlersRef.current = [];
        connectWebSocket();

        return () => {
            intentionalCloseRef.current = true;
            if (reconnectTimerRef.current) {
                clearTimeout(reconnectTimerRef.current);
                reconnectTimerRef.current = null;
            }
            if (pingTimerRef.current) {
                clearInterval(pingTimerRef.current);
                pingTimerRef.current = null;
            }
            if (wsRef.current) {
                wsRef.current.close();
                wsRef.current = null;
            }
        };
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [session, sessionId]);

    // -- WebSocket Event Handler (default/fallback mode) ---------

    const handleWSEvent = useCallback((event: WSEvent) => {
        switch (event.event) {
            case 'agent_thinking': {
                const payload = event.payload as { status: string };
                setThinkingStatus(payload.status);
                break;
            }

            case 'token_delta': {
                const payload = event.payload as { text: string };
                setThinkingStatus(null);
                setStreamingContent(prev => prev + payload.text);
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

                // Show a system message about the scaffold change
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

            case 'turn_complete': {
                setThinkingStatus(null);
                setSending(false);

                // Flush streaming content to a message
                setStreamingContent(prev => {
                    if (prev) {
                        setMessages(msgs => [
                            ...msgs,
                            {
                                id: `coach-${Date.now()}`,
                                role: 'coach',
                                content: prev,
                                timestamp: Date.now(),
                            },
                        ]);
                    }
                    return '';
                });

                inputRef.current?.focus();
                break;
            }

            case 'error': {
                const payload = event.payload as { message: string };
                setThinkingStatus(null);
                setSending(false);
                alert(payload.message);
                break;
            }
        }
    }, []);

    // -- Send Message (fallback mode) ----------------------------

    const handleSend = useCallback(() => {
        const text = input.trim();
        if (!text || sending || !wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return;

        // Add student message to chat
        setMessages(prev => [
            ...prev,
            {
                id: `student-${Date.now()}`,
                role: 'student',
                content: text,
                timestamp: Date.now(),
            },
        ]);

        // Send via WebSocket
        const event: WSEvent = {
            event: 'user_message',
            payload: { text },
            timestamp: Math.floor(Date.now() / 1000),
        };
        wsRef.current.send(JSON.stringify(event));

        setInput('');
        setSending(true);
        setStreamingContent('');

        // Reset textarea height after send
        if (inputRef.current) {
            inputRef.current.style.height = 'auto';
        }
    }, [input, sending]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        }
    };

    // -- Auto-resize Textarea ------------------------------------

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
        setInput(e.target.value);
        const textarea = e.target;
        textarea.style.height = 'auto';
        textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
    }, []);

    // -- Render Scaffold UI (T-4.13) — fallback mode only --------

    const renderScaffold = () => {
        switch (scaffoldLevel) {
            case 'high':
                return renderScaffoldHigh();
            case 'medium':
                return renderScaffoldMedium();
            case 'low':
                return null; // Low scaffold = blank input only
        }
    };

    const renderScaffoldHigh = () => {
        const steps = scaffoldData.steps || [
            '仔细阅读问题，理解要求',
            '回忆相关的知识点和概念',
            '尝试用自己的语言描述思路',
            '逐步推导或分析',
        ];
        const keywords = scaffoldData.keywords || [];

        return (
            <div className={`${styles.scaffoldPanel} ${scaffoldTransition ? styles.scaffoldTransition : ''}`}>
                <div className={styles.scaffoldPanelHeader}>
                    分步引导
                </div>
                <div className={styles.scaffoldSteps}>
                    {steps.map((step, i) => (
                        <div key={i} className={styles.scaffoldStep}>
                            <span className={styles.stepNumber}>{i + 1}</span>
                            <span>{step}</span>
                        </div>
                    ))}
                </div>
                {keywords.length > 0 && (
                    <div className={styles.scaffoldTags}>
                        {keywords.map((kw, i) => (
                            <span key={i} className={styles.keywordHighlight}>{kw}</span>
                        ))}
                    </div>
                )}
            </div>
        );
    };

    const renderScaffoldMedium = () => {
        const keywords = scaffoldData.keywords || ['关键概念', '前置知识', '核心思路'];

        return (
            <div className={`${styles.scaffoldTags} ${scaffoldTransition ? styles.scaffoldTransition : ''}`}>
                {keywords.map((kw, i) => (
                    <span key={i} className={styles.scaffoldTag}>{kw}</span>
                ))}
            </div>
        );
    };

    // -- Render Agent Thinking (T-4.14) --------------------------

    const renderThinking = () => {
        if (!thinkingStatus) return null;
        return (
            <div className={styles.thinkingIndicator}>
                <div className={styles.thinkingDots}>
                    <div className={styles.thinkingDot} />
                    <div className={styles.thinkingDot} />
                    <div className={styles.thinkingDot} />
                </div>
                <span>{thinkingStatus}</span>
            </div>
        );
    };

    // -- Render Connection Status ---------------------------------

    const renderConnectionStatus = () => {
        if (wsStatus === 'connected') return null; // Don't show when healthy
        const labels: Record<WSStatus, string> = {
            connecting: '连接中...',
            connected: '已连接',
            reconnecting: `重连中 (${reconnectCountRef.current}/${WS_RECONNECT_MAX_RETRIES})...`,
            disconnected: '连接断开',
        };
        return (
            <div className={`${styles.connectionStatus} ${styles[`ws_${wsStatus}`]}`}>
                <span className={styles.connectionDot} />
                {labels[wsStatus]}
            </div>
        );
    };

    // -- Default Chat Interface (fallback when no skill renderer) -

    const renderDefaultChat = () => (
        <>
            {/* Messages */}
            <div className={styles.messagesArea}>
                {messages.length === 0 && !thinkingStatus && (
                    <div className={styles.messageSystem}>
                        发送消息开始学习对话
                    </div>
                )}

                {messages.map(msg => (
                    <div
                        key={msg.id}
                        className={`${styles.messageBubble} ${
                            msg.role === 'student' ? styles.messageStudent :
                            msg.role === 'coach' ? styles.messageCoach :
                            styles.messageSystem
                        }`}
                    >
                        {msg.role !== 'system' && (
                            <div className={styles.messageHeader}>
                                <span className={`${styles.roleIcon} ${
                                    msg.role === 'student' ? styles.roleStudent : styles.roleCoach
                                }`}>
                                    {msg.role === 'student' ? 'S' : 'AI'}
                                </span>
                                <span className={styles.roleLabel}>
                                    {msg.role === 'student' ? '我' : 'AI 导师'}
                                </span>
                            </div>
                        )}
                        <div className={styles.messageContent}>
                            {msg.role === 'coach' ? (
                                <MarkdownRenderer content={msg.content} />
                            ) : (
                                msg.content
                            )}
                        </div>
                    </div>
                ))}

                {/* Streaming content (partial coach response) */}
                {streamingContent && (
                    <div className={`${styles.messageBubble} ${styles.messageCoach}`}>
                        <div className={styles.messageHeader}>
                            <span className={`${styles.roleIcon} ${styles.roleCoach}`}>AI</span>
                            <span className={styles.roleLabel}>AI 导师</span>
                        </div>
                        <div className={styles.messageContent}>
                            <MarkdownRenderer content={streamingContent} isStreaming />
                        </div>
                    </div>
                )}

                {/* Agent thinking indicator */}
                {renderThinking()}

                <div ref={messagesEndRef} />
            </div>

            {/* Scaffold UI */}
            {renderScaffold()}

            {/* Input */}
            <div className={styles.inputArea}>
                <textarea
                    ref={inputRef}
                    className={styles.chatInput}
                    value={input}
                    onChange={handleInputChange}
                    onKeyDown={handleKeyDown}
                    placeholder={
                        session!.status !== 'active'
                            ? '会话已结束'
                            : sending
                            ? 'AI 正在思考...'
                            : '输入你的想法或问题... (Enter 发送, Shift+Enter 换行)'
                    }
                    disabled={session!.status !== 'active' || sending}
                    rows={1}
                />
                <button
                    className={`btn btn-primary ${styles.sendBtn}`}
                    onClick={handleSend}
                    disabled={!input.trim() || sending || session!.status !== 'active'}
                >
                    发送
                </button>
            </div>
        </>
    );

    // -- Render Skill Renderer (plugin mode) ---------------------

    const renderSkillRenderer = () => {
        if (!matchedPlugin || !session) return null;
        const RendererComponent = matchedPlugin.Component as unknown as React.FC<SkillRendererProps>;
        const rendererProps: SkillRendererProps = {
            studentContext: {
                studentId: session.student_id,
                displayName: '', // Not available in session data
                courseId: 0, // Will be enhanced when API provides this
                sessionId: session.id,
            },
            knowledgePoint: {
                id: session.current_kp_id,
                title: '', // Will be enhanced when API provides this
                difficulty: 0,
                chapterTitle: '',
            },
            scaffoldingLevel: scaffoldLevel,
            agentChannel,
            onInteractionEvent: handleInteractionEvent,
        };
        return <RendererComponent {...rendererProps} />;
    };

    // -- Loading State -------------------------------------------

    if (loading) {
        return (
            <div style={{ display: 'flex', justifyContent: 'center', padding: '80px 0' }}>
                <div className="spinner" />
            </div>
        );
    }

    if (!session) {
        return (
            <div style={{ textAlign: 'center', padding: '80px 0', color: 'var(--text-muted)' }}>
                会话不存在
            </div>
        );
    }

    // -- Main Render ---------------------------------------------

    return (
        <div className="fade-in">
            <div className={styles.chatContainer}>
                {/* Header */}
                <div className={styles.chatHeader}>
                    <div>
                        <Link href="/student/activities" className={styles.backLink}>
                            &larr; 返回活动列表
                        </Link>
                        <div className={styles.chatTitle}>AI 学习对话</div>
                    </div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                        {renderConnectionStatus()}
                        <div className={`${styles.scaffoldBadge} ${
                            scaffoldLevel === 'high' ? styles.scaffoldHigh :
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
                {matchedPlugin ? renderSkillRenderer() : renderDefaultChat()}
            </div>
        </div>
    );
}
