'use client';

/**
 * Socratic Questioning Skill Renderer.
 *
 * Renders a multi-round conversational UI with scaffold-aware
 * hints: high = step-by-step panel + keywords + fill-in-blank,
 * medium = keyword tags, low = pure blank input.
 *
 * Per design.md §7.13 — Conversational sub-category.
 */

import { useState, useRef, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import ChatInputArea from '@/components/ChatInputArea';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './SocraticRenderer.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
}

interface ScaffoldData {
    steps?: string[];
    keywords?: string[];
    fillBlank?: string;
}

// -- Component ---------------------------------------------------

export default function SocraticRenderer({
    knowledgePoint,
    scaffoldingLevel,
    agentChannel,
    onInteractionEvent,
}: SkillRendererProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
    const [streamingContent, setStreamingContent] = useState('');
    const [scaffoldData, setScaffoldData] = useState<ScaffoldData>({});
    const [scaffoldTransition, setScaffoldTransition] = useState(false);

    const messagesEndRef = useRef<HTMLDivElement>(null);

    // -- Scroll to bottom ----------------------------------------

    const scrollToBottom = useCallback(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, []);

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingContent, thinkingStatus, scrollToBottom]);

    // -- WebSocket message handling ------------------------------

    useEffect(() => {
        const unsubscribeMessage = agentChannel.onMessage((data: string) => {
            try {
                const event = JSON.parse(data);
                switch (event.event) {
                    case 'agent_thinking': {
                        setThinkingStatus(event.payload?.status || '思考中...');
                        break;
                    }
                    case 'token_delta': {
                        setThinkingStatus(null);
                        setStreamingContent(prev => prev + (event.payload?.text || ''));
                        break;
                    }
                    case 'ui_scaffold_change': {
                        const payload = event.payload as {
                            data: {
                                new_level: 'high' | 'medium' | 'low';
                                mastery: number;
                                direction: string;
                            };
                        };
                        setScaffoldTransition(true);

                        const direction = payload.data.direction === 'fade' ? '降低' : '增强';
                        const labels = { high: '高支架', medium: '中支架', low: '低支架' };
                        setMessages(prev => [...prev, {
                            id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                            role: 'system',
                            content: `支架已${direction}至 ${labels[payload.data.new_level]} (掌握度: ${(payload.data.mastery * 100).toFixed(0)}%)`,
                            timestamp: Date.now(),
                        }]);

                        setTimeout(() => setScaffoldTransition(false), 500);
                        break;
                    }
                    case 'turn_complete': {
                        setThinkingStatus(null);
                        setSending(false);
                        setStreamingContent(prev => {
                            if (prev) {
                                setMessages(msgs => [...msgs, {
                                    id: `coach-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                                    role: 'coach',
                                    content: prev,
                                    timestamp: Date.now(),
                                }]);
                            }
                            return '';
                        });
                        // Extract scaffold data from turn_complete payload if provided
                        if (event.payload?.scaffold_data) {
                            setScaffoldData(event.payload.scaffold_data);
                        }
                        break;
                    }
                    case 'error': {
                        setThinkingStatus(null);
                        setSending(false);
                        setMessages(prev => [...prev, {
                            id: `err-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                            role: 'system',
                            content: event.payload?.message || '发生错误',
                            timestamp: Date.now(),
                        }]);
                        break;
                    }
                }
            } catch {
                // Ignore parse errors
            }
        });

        const unsubscribeClose = agentChannel.onClose(() => {
            setMessages(prev => [...prev, {
                id: `sys-close-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                role: 'system',
                content: '连接已断开',
                timestamp: Date.now(),
            }]);
        });
        return () => {
            unsubscribeMessage();
            unsubscribeClose();
        };
    }, [agentChannel]);

    // -- Send message --------------------------------------------

    const handleSend = useCallback(() => {
        const text = input.trim();
        if (!text || sending) return;

        setMessages(prev => [...prev, {
            id: `student-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
            role: 'student',
            content: text,
            timestamp: Date.now(),
        }]);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        onInteractionEvent({
            type: 'message_sent',
            payload: { skillId: 'general_concept_socratic', length: text.length },
            timestamp: Date.now(),
        });

        setInput('');
        setSending(true);
        setStreamingContent('');
    }, [input, sending, agentChannel, onInteractionEvent]);

    

    // -- Render Scaffold UI --------------------------------------

    const renderScaffold = () => {
        switch (scaffoldingLevel) {
            case 'high':
                return renderScaffoldHigh();
            case 'medium':
                return renderScaffoldMedium();
            case 'low':
                return null;
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
            <>
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
                        <div className={styles.scaffoldTags} style={{ borderTop: 'none', marginTop: 0, padding: '8px 16px' }}>
                            {keywords.map((kw, i) => (
                                <span key={i} className={styles.keywordHighlight}>{kw}</span>
                            ))}
                        </div>
                    )}
                </div>
                {scaffoldData.fillBlank && (
                    <div className={styles.fillBlankArea}>
                        <div className={styles.fillBlankLabel}>填空练习</div>
                        <div className={styles.fillBlankSentence}>
                            {scaffoldData.fillBlank.split('___').map((part, i, arr) => (
                                <span key={i}>
                                    {part}
                                    {i < arr.length - 1 && <span className={styles.fillBlankSlot}>?</span>}
                                </span>
                            ))}
                        </div>
                    </div>
                )}
            </>
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

    // -- Render --------------------------------------------------

    return (
        <div className={styles.container}>
            {/* KP context header */}
            {knowledgePoint.title && (
                <div className={styles.kpHeader}>
                    <span className={styles.kpBadge}>知识点</span>
                    <span className={styles.kpTitle}>{knowledgePoint.title}</span>
                </div>
            )}

            {/* Messages */}
            <div className={styles.messagesArea}>
                {messages.length === 0 && !thinkingStatus && (
                    <div className={styles.emptyHint}>
                        发送消息开始苏格拉底式对话
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
                            ) : msg.content}
                        </div>
                    </div>
                ))}

                {/* Streaming content */}
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

                {/* Thinking indicator */}
                {thinkingStatus && (
                    <div className={styles.thinkingIndicator}>
                        <div className={styles.thinkingDots}>
                            <div className={styles.thinkingDot} />
                            <div className={styles.thinkingDot} />
                            <div className={styles.thinkingDot} />
                        </div>
                        <span>{thinkingStatus}</span>
                    </div>
                )}

                <div ref={messagesEndRef} />
            </div>

            {/* Scaffold UI */}
            {renderScaffold()}

            {/* Input */}
            <ChatInputArea
                input={input}
                setInput={setInput}
                sending={sending}
                onSend={() => handleSend()}
                placeholder={sending ? '苏格拉底正在思考...' : '输入消息...'}
            />
        </div>
    );
}