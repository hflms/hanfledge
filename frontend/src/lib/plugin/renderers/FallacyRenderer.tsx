'use client';

/**
 * Fallacy Detective Skill Renderer.
 *
 * Challenge-style UI with misconception cards, text highlight
 * tool for marking errors, reasoning input, and a reveal
 * animation when the fallacy is correctly identified.
 *
 * Per design.md §7.13 — Challenge sub-category.
 */

import { useState, useRef, useCallback, useEffect } from 'react';
import type { SkillRendererProps } from '@/lib/plugin/types';
import MarkdownRenderer from '@/components/MarkdownRenderer';
import styles from './FallacyRenderer.module.css';

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
}

/** Fallacy phase names matching backend FallacySessionState */
type FallacyPhase = 'present' | 'identify' | 'explain' | 'correct' | 'reflect';

const PHASE_LABELS: Record<FallacyPhase, string> = {
    present: '呈现谬误',
    identify: '识别错误',
    explain: '解释原因',
    correct: '纠正表述',
    reflect: '反思总结',
};

const PHASE_ORDER: FallacyPhase[] = ['present', 'identify', 'explain', 'correct', 'reflect'];

interface RevealData {
    title: string;
    explanation: string;
}

// -- Component ---------------------------------------------------

export default function FallacyRenderer({
    studentContext: _studentContext,
    knowledgePoint,
    scaffoldingLevel: _scaffoldingLevel,
    agentChannel,
    onInteractionEvent,
}: SkillRendererProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
    const [streamingContent, setStreamingContent] = useState('');

    // Fallacy-specific state
    const [currentPhase, setCurrentPhase] = useState<FallacyPhase>('present');
    const [misconceptionText, setMisconceptionText] = useState('');
    const [highlightedRanges, setHighlightedRanges] = useState<Array<{ start: number; end: number }>>([]);
    const [revealData, setRevealData] = useState<RevealData | null>(null);

    const messagesEndRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLTextAreaElement>(null);
    const statementRef = useRef<HTMLDivElement>(null);

    // -- Scroll to bottom ----------------------------------------

    const scrollToBottom = useCallback(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, []);

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingContent, thinkingStatus, scrollToBottom]);

    // -- WebSocket message handling ------------------------------

    useEffect(() => {
        agentChannel.onMessage((data: string) => {
            try {
                const event = JSON.parse(data);
                switch (event.event) {
                    case 'agent_thinking': {
                        setThinkingStatus(event.payload?.status || '分析中...');
                        break;
                    }
                    case 'token_delta': {
                        setThinkingStatus(null);
                        setStreamingContent(prev => prev + (event.payload?.text || ''));
                        break;
                    }
                    case 'turn_complete': {
                        setThinkingStatus(null);
                        setSending(false);
                        setStreamingContent(prev => {
                            if (prev) {
                                setMessages(msgs => [...msgs, {
                                    id: `coach-${Date.now()}`,
                                    role: 'coach',
                                    content: prev,
                                    timestamp: Date.now(),
                                }]);
                            }
                            return '';
                        });
                        // Update phase if provided
                        if (event.payload?.fallacy_phase) {
                            setCurrentPhase(event.payload.fallacy_phase);
                        }
                        // Update misconception text if provided
                        if (event.payload?.misconception_text) {
                            setMisconceptionText(event.payload.misconception_text);
                            setHighlightedRanges([]);
                        }
                        inputRef.current?.focus();
                        break;
                    }
                    case 'fallacy_identified': {
                        // Reveal animation when student correctly identifies the fallacy
                        setRevealData({
                            title: event.payload?.title || '正确识别了谬误!',
                            explanation: event.payload?.explanation || '',
                        });
                        onInteractionEvent({
                            type: 'fallacy_identified',
                            payload: { skillId: 'general_assessment_fallacy', phase: currentPhase },
                            timestamp: Date.now(),
                        });
                        break;
                    }
                    case 'ui_scaffold_change': {
                        const payload = event.payload as {
                            data: { new_level: string; mastery: number; direction: string };
                        };
                        const direction = payload.data.direction === 'fade' ? '降低' : '增强';
                        const labels = { high: '高支架', medium: '中支架', low: '低支架' };
                        const label = labels[payload.data.new_level as keyof typeof labels] || payload.data.new_level;
                        setMessages(prev => [...prev, {
                            id: `sys-${Date.now()}`,
                            role: 'system',
                            content: `支架已${direction}至 ${label} (掌握度: ${(payload.data.mastery * 100).toFixed(0)}%)`,
                            timestamp: Date.now(),
                        }]);
                        break;
                    }
                    case 'error': {
                        setThinkingStatus(null);
                        setSending(false);
                        setMessages(prev => [...prev, {
                            id: `err-${Date.now()}`,
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

        agentChannel.onClose(() => {
            setMessages(prev => [...prev, {
                id: `sys-close-${Date.now()}`,
                role: 'system',
                content: '连接已断开',
                timestamp: Date.now(),
            }]);
        });
    }, [agentChannel, currentPhase, onInteractionEvent]);

    // -- Text highlight handling ---------------------------------

    const handleTextSelect = useCallback(() => {
        if (!statementRef.current) return;
        const selection = window.getSelection();
        if (!selection || selection.isCollapsed) return;

        const range = selection.getRangeAt(0);
        const container = statementRef.current;

        // Only handle selections within the statement
        if (!container.contains(range.commonAncestorContainer)) return;

        // Calculate offsets relative to the statement text
        const fullText = misconceptionText;
        const selectedText = selection.toString();
        const startIdx = fullText.indexOf(selectedText);

        if (startIdx >= 0) {
            setHighlightedRanges(prev => {
                // Avoid duplicate ranges
                const exists = prev.some(r => r.start === startIdx && r.end === startIdx + selectedText.length);
                if (exists) return prev;
                return [...prev, { start: startIdx, end: startIdx + selectedText.length }];
            });

            onInteractionEvent({
                type: 'text_highlighted',
                payload: { text: selectedText, start: startIdx },
                timestamp: Date.now(),
            });
        }

        selection.removeAllRanges();
    }, [misconceptionText, onInteractionEvent]);

    // -- Render misconception statement with highlights -----------

    const renderMisconceptionText = () => {
        if (!misconceptionText) return null;

        if (highlightedRanges.length === 0) {
            return <span>{misconceptionText}</span>;
        }

        // Sort ranges and render with highlights
        const sorted = [...highlightedRanges].sort((a, b) => a.start - b.start);
        const parts: React.ReactNode[] = [];
        let lastEnd = 0;

        sorted.forEach((range, i) => {
            if (range.start > lastEnd) {
                parts.push(<span key={`t-${i}`}>{misconceptionText.slice(lastEnd, range.start)}</span>);
            }
            parts.push(
                <span key={`h-${i}`} className={styles.highlightedText}>
                    {misconceptionText.slice(range.start, range.end)}
                </span>
            );
            lastEnd = range.end;
        });

        if (lastEnd < misconceptionText.length) {
            parts.push(<span key="tail">{misconceptionText.slice(lastEnd)}</span>);
        }

        return <>{parts}</>;
    };

    // -- Send message --------------------------------------------

    const handleSend = useCallback(() => {
        const text = input.trim();
        if (!text || sending) return;

        // Include highlighted ranges in the message payload for context
        const payload: Record<string, unknown> = { text };
        if (highlightedRanges.length > 0) {
            payload.highlighted = highlightedRanges.map(r => misconceptionText.slice(r.start, r.end));
        }

        setMessages(prev => [...prev, {
            id: `student-${Date.now()}`,
            role: 'student',
            content: text,
            timestamp: Date.now(),
        }]);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload,
            timestamp: Math.floor(Date.now() / 1000),
        }));

        onInteractionEvent({
            type: 'reasoning_submitted',
            payload: { skillId: 'general_assessment_fallacy', phase: currentPhase, length: text.length },
            timestamp: Date.now(),
        });

        setInput('');
        setSending(true);
        setStreamingContent('');

        if (inputRef.current) {
            inputRef.current.style.height = 'auto';
        }
    }, [input, sending, agentChannel, onInteractionEvent, currentPhase, highlightedRanges, misconceptionText]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleSend();
        }
    };

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
        setInput(e.target.value);
        const textarea = e.target;
        textarea.style.height = 'auto';
        textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
    }, []);

    // -- Phase indicator -----------------------------------------

    const currentPhaseIdx = PHASE_ORDER.indexOf(currentPhase);

    // -- Render placeholders for input based on phase ------------

    const getInputPlaceholder = (): string => {
        if (sending) return 'AI 正在分析...';
        switch (currentPhase) {
            case 'present': return '阅读上面的表述，你觉得哪里有问题？';
            case 'identify': return '指出你认为错误的部分，并说明为什么...';
            case 'explain': return '解释为什么这个表述是错误的...';
            case 'correct': return '写出正确的表述...';
            case 'reflect': return '总结你从中学到了什么...';
        }
    };

    // -- Render --------------------------------------------------

    return (
        <div className={styles.container}>
            {/* KP context header */}
            {knowledgePoint.title && (
                <div className={styles.kpHeader}>
                    <span className={styles.kpBadge}>谬误侦探</span>
                    <span className={styles.kpTitle}>{knowledgePoint.title}</span>
                </div>
            )}

            {/* Phase progress indicator */}
            <div className={styles.phaseBar}>
                <span className={styles.phaseLabel}>阶段</span>
                <span className={styles.phaseName}>{PHASE_LABELS[currentPhase]}</span>
                <div className={styles.phaseSteps}>
                    {PHASE_ORDER.map((phase, i) => (
                        <div
                            key={phase}
                            className={`${styles.phaseStep} ${
                                i < currentPhaseIdx ? styles.phaseStepDone :
                                i === currentPhaseIdx ? styles.phaseStepActive : ''
                            }`}
                        />
                    ))}
                </div>
            </div>

            {/* Misconception card */}
            {misconceptionText && (
                <div className={styles.misconceptionCard}>
                    <div className={styles.misconceptionCardHeader}>
                        <span className={styles.misconceptionIcon}>&#x1F50D;</span>
                        <span className={styles.misconceptionTitle}>
                            {currentPhase === 'present' ? '请审阅以下表述' : '标记的表述'}
                        </span>
                    </div>
                    <div
                        ref={statementRef}
                        className={styles.misconceptionStatement}
                        onMouseUp={handleTextSelect}
                    >
                        {renderMisconceptionText()}
                    </div>
                    {currentPhase === 'identify' && highlightedRanges.length === 0 && (
                        <div className={styles.highlightHint}>
                            提示: 选中你认为有错误的文字来标记
                        </div>
                    )}
                </div>
            )}

            {/* Messages */}
            <div className={styles.messagesArea}>
                {messages.length === 0 && !thinkingStatus && !misconceptionText && (
                    <div className={styles.emptyHint}>
                        等待谬误侦探出题...
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
                                    {msg.role === 'student' ? 'S' : 'FD'}
                                </span>
                                <span className={styles.roleLabel}>
                                    {msg.role === 'student' ? '我' : '谬误侦探'}
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

                {/* Streaming */}
                {streamingContent && (
                    <div className={`${styles.messageBubble} ${styles.messageCoach}`}>
                        <div className={styles.messageHeader}>
                            <span className={`${styles.roleIcon} ${styles.roleCoach}`}>FD</span>
                            <span className={styles.roleLabel}>谬误侦探</span>
                        </div>
                        <div className={styles.messageContent}>
                            <MarkdownRenderer content={streamingContent} isStreaming />
                        </div>
                    </div>
                )}

                {/* Thinking */}
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

            {/* Reasoning input */}
            <div className={styles.reasoningArea}>
                <div className={styles.reasoningLabel}>
                    {currentPhase === 'identify' ? '你的判断' :
                     currentPhase === 'explain' ? '你的解释' :
                     currentPhase === 'correct' ? '正确表述' :
                     '你的回答'}
                </div>
                <div className={styles.inputArea}>
                    <textarea
                        ref={inputRef}
                        className={styles.chatInput}
                        value={input}
                        onChange={handleInputChange}
                        onKeyDown={handleKeyDown}
                        placeholder={getInputPlaceholder()}
                        disabled={sending}
                        rows={1}
                    />
                    <button
                        className={`btn btn-primary ${styles.sendBtn}`}
                        onClick={handleSend}
                        disabled={!input.trim() || sending}
                    >
                        提交
                    </button>
                </div>
            </div>

            {/* Reveal overlay */}
            {revealData && (
                <div className={styles.revealOverlay} onClick={() => setRevealData(null)}>
                    <div className={styles.revealCard} onClick={e => e.stopPropagation()}>
                        <div className={styles.revealIcon}>&#x2705;</div>
                        <div className={styles.revealTitle}>{revealData.title}</div>
                        <div className={styles.revealExplanation}>
                            <MarkdownRenderer content={revealData.explanation} />
                        </div>
                        <button className={styles.revealCloseBtn} onClick={() => setRevealData(null)}>
                            继续学习
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
