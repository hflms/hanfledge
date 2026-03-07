'use client';

import { useEffect, useState, useRef, useCallback, useMemo } from 'react';
import { useParams, useRouter } from 'next/navigation';
import Link from 'next/link';

import {
    getSession,
    updateSessionStep,
    getSystemConfig,
    type LearningActivity,
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
    const [activity, setActivity] = useState<LearningActivity | null>(null);
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
        switch (event.event) {
            // Skill renderer 处理的事件 - 如果有 activePlugin 则跳过
            case 'agent_thinking':
            case 'token_delta':
            case 'turn_complete': {
                if (activePlugin) return; // Skill renderer 会处理这些事件
                
                // 原有逻辑
                if (event.event === 'agent_thinking') {
                    const payload = event.payload as { status: string };
                    setThinkingStatus(payload.status);
                } else if (event.event === 'token_delta') {
                    const payload = event.payload as { text?: string; content?: string; delta?: string };
                    const delta = payload.text ?? payload.content ?? payload.delta ?? '';
                    if (!delta) break;
                    setThinkingStatus(null);
                    setStreamingContent(prev => {
                        const next = prev + delta;
                        lastResponseRef.current = next;
                        return next;
                    });
                } else if (event.event === 'turn_complete') {
                    console.log('[SESSION DEBUG] ✅ turn_complete 收到, 流式内容长度:', streamingContent.length);
                    setThinkingStatus(null);
                    setSending(false);

                    setStreamingContent(prev => {
                        const content = prev || lastResponseRef.current;
                        if (content) {
                            setMessages(msgs => [
                                ...msgs,
                                {
                                    id: `coach-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
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
                }
                break;
            }

            case 'ui_scaffold_change': {
                const payload = event.payload as {
                    action: string;
                    data: any; // Allow mixed types
                };

                if (payload.action === 'skill_change') {
                    const newSkill = payload.data.new_skill;
                    setSession(prev => prev ? { ...prev, active_skill: newSkill } : null);
                    toast(`系统已根据您的掌握度自动切换到技能: ${newSkill}`, 'success');
                    break;
                }

                setScaffoldTransition(true);
                setScaffoldLevel(payload.data.new_level as ScaffoldLevel);

                const direction = payload.data.direction === 'fade' ? '降低' : '增强';
                const newLabel = SCAFFOLD_LABELS[payload.data.new_level as ScaffoldLevel];
                setMessages(prev => [
                    ...prev,
                    {
                        id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
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
                        id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                        role: 'system',
                        content: payload.content,
                        timestamp: Date.now(),
                    }
                ]);
                break;
            }

            case 'teacher_takeover': {
                // 教师接管事件 - 始终由 page.tsx 处理
                const payload = event.payload as { id: number, content: string, created_at: string };
                setMessages(prev => [
                    ...prev,
                    {
                        id: `t-${payload.id || Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
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

    // -- Pipeline Steps Computation ---------------------------------

    const steps = useMemo(() => {
        if (!activity) return [];
        try {
            const kpIds: number[] = JSON.parse(activity.kp_ids || '[]');
            const config = JSON.parse(activity.skill_config || '{}');
            const defaultSkill = config.default_skill || 'concept-explain';
            
            return kpIds.map((kpId, index) => {
                const stepSkill = config[kpId] || defaultSkill;
                return {
                    index,
                    kpId,
                    skill: stepSkill,
                    label: `步骤 ${index + 1}`
                };
            });
        } catch (e) {
            console.error('Failed to parse activity config', e);
            return [];
        }
    }, [activity]);

    const currentStepIndex = useMemo(() => {
        if (!session || steps.length === 0) return 0;
        const idx = steps.findIndex(s => s.kpId === session.current_kp_id);
        return idx >= 0 ? idx : 0;
    }, [session, steps]);

    const handleNextStep = async () => {
        if (currentStepIndex < steps.length - 1) {
            const nextStep = steps[currentStepIndex + 1];
            try {
                await updateSessionStep(sessionId, nextStep.kpId, nextStep.skill);
                setSession(prev => prev ? { ...prev, current_kp_id: nextStep.kpId, active_skill: nextStep.skill } : null);
                
                // Clear messages to trigger auto-start in the new renderer
                setMessages([]);
                
                // Optional: send a kickstart message immediately via WS
                agentChannel.send(JSON.stringify({
                    event: 'user_message',
                    payload: { text: `[系统] 学生已进入下一步学习阶段：${nextStep.label}。请根据当前知识点和技能重新开始引导。` },
                    timestamp: Math.floor(Date.now() / 1000)
                }));

                toast('已进入下一步', 'success');
            } catch (err) {
                toast('切换步骤失败', 'error');
            }
        }
    };

    const handlePrevStep = async () => {
        if (currentStepIndex > 0) {
            const prevStep = steps[currentStepIndex - 1];
            try {
                await updateSessionStep(sessionId, prevStep.kpId, prevStep.skill);
                setSession(prev => prev ? { ...prev, current_kp_id: prevStep.kpId, active_skill: prevStep.skill } : null);
                
                // Clear messages to trigger auto-start in the new renderer
                setMessages([]);
                
                // Optional: send a kickstart message immediately via WS
                agentChannel.send(JSON.stringify({
                    event: 'user_message',
                    payload: { text: `[系统] 学生已返回上一步学习阶段：${prevStep.label}。请恢复该阶段的引导。` },
                    timestamp: Math.floor(Date.now() / 1000)
                }));

                toast('已返回上一步', 'success');
            } catch (err) {
                toast('切换步骤失败', 'error');
            }
        }
    };

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

    const autoStartTriggeredRef = useRef(false);

    useEffect(() => {
        if (!sessionId) return;

        const loadSession = async () => {
            try {
                const data = await getSession(sessionId);
                setSession(data.session);
                setActivity(data.activity);
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

                // 自动开始: 如果是新会话(无历史消息),标记需要自动开始
                if (existingMessages.length === 0 && data.session.status === 'active' && !autoStartTriggeredRef.current) {
                    autoStartTriggeredRef.current = true;
                    console.log('[SESSION] 新会话,准备自动开始学习活动');
                }
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

    // -- Auto-start session when WebSocket is ready ----------------

    useEffect(() => {
        if (autoStartTriggeredRef.current && wsStatus === 'connected' && !sending) {
            console.log('[SESSION] WebSocket 已连接,自动发送开始消息');
            agentChannel.send(JSON.stringify({
                event: 'user_message',
                payload: { text: '开始学习' },
                timestamp: Math.floor(Date.now() / 1000),
            }));
            setSending(true);
            autoStartTriggeredRef.current = false; // 防止重复触发
        }
    }, [wsStatus, agentChannel, sending]);

    // -- Send Message -----------------------------------------------

    const handleSend = useCallback(async (textOverride?: string | React.MouseEvent) => {
        const overrideStr = typeof textOverride === 'string' ? textOverride : undefined;
        const text = (overrideStr || input).trim();
        if (!text || sending) return;
        if (wsStatus !== 'connected') {
            toast('连接未就绪，请稍后重试', 'warning');
            return;
        }

        setMessages(prev => [
            ...prev,
            {
                id: `student-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                role: 'student',
                content: text,
                timestamp: Date.now(),
            },
        ]);

        if (!textOverride) {
            setInput('');
        }

        // L1 Cache: check for cached response before sending
        const cached = await getCachedResponse(sessionId, text);
        if (cached) {
            console.log('[L1 Cache] Hit — returning cached response');
            setMessages(prev => [
                ...prev,
                {
                    id: `coach-cache-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
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
            initialMessages: messages,
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

                {/* Steps Navigation */}
                {steps.length > 1 && (
                    <div className={styles.stepsNav}>
                        <button 
                            disabled={currentStepIndex === 0} 
                            onClick={handlePrevStep}
                            className={styles.stepBtn}
                        >
                            上一步
                        </button>
                        <div className={styles.stepIndicator}>
                            {steps.map((step, idx) => (
                                <span 
                                    key={idx} 
                                    className={`${styles.stepDot} ${idx === currentStepIndex ? styles.stepDotActive : ''} ${idx < currentStepIndex ? styles.stepDotCompleted : ''}`}
                                    title={step.skill}
                                />
                            ))}
                        </div>
                        <span className={styles.stepLabel}>
                            {steps[currentStepIndex]?.label || ''}
                        </span>
                        <button 
                            disabled={currentStepIndex === steps.length - 1} 
                            onClick={handleNextStep}
                            className={styles.stepBtn}
                        >
                            下一步
                        </button>
                    </div>
                )}

                {/* Skill Renderer or Default Chat */}
                {activePlugin ? renderSkillRenderer() : (
                    <>
                        <MessageList
                            messages={messages}
                            streamingContent={streamingContent}
                            thinkingStatus={thinkingStatus}
                            onSurveySelect={(text) => {
                                setInput(prev => {
                                    const next = prev.trim();
                                    const match = text.match(/^Q(\d+):\s*/);
                                    if (!next) return text;
                                    if (match) {
                                        const questionKey = `Q${match[1]}:`;
                                        const parts = next
                                            .split(';')
                                            .map(part => part.trim())
                                            .filter(Boolean);
                                        const existingIndex = parts.findIndex(part => part.startsWith(questionKey));
                                        if (existingIndex >= 0) {
                                            parts[existingIndex] = text;
                                            return parts.join('; ');
                                        }
                                        return `${next}; ${text}`;
                                    }
                                    return `${next}; ${text}`;
                                });
                            }}
                            onQuickReply={(text) => {
                                handleSend(text);
                            }}
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
