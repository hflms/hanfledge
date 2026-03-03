'use client';

/**
 * Role-Play Skill Renderer.
 *
 * Immersive dialog UI with character avatar cards, scenario
 * panel (background + objectives), character-toned chat
 * bubbles, and an exit flow that triggers a learning summary.
 *
 * Per design.md §7.13 — Immersive sub-category.
 */

import { useState, useRef, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import type { SkillRendererProps } from '@/lib/plugin/types';
import { useModalA11y, cardA11yProps } from '@/lib/a11y';
import styles from './RolePlayRenderer.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
    characterName?: string;
    tone?: string;
}

interface CharacterInfo {
    name: string;
    role: string;
    avatar: string;
}

interface ScenarioInfo {
    title: string;
    description: string;
    objectives: string[];
    switches: number;
    maxSwitches: number;
}

// -- Default characters per subject (matching backend SKILL.md) --

const DEFAULT_CHARACTERS: CharacterInfo[] = [
    { name: '苏格拉底', role: '哲学家', avatar: '\u{1F9D4}' },
    { name: '爱因斯坦', role: '物理学家', avatar: '\u{1F468}\u{200D}\u{1F52C}' },
    { name: '李白', role: '诗人', avatar: '\u{1F3A8}' },
    { name: '居里夫人', role: '化学家', avatar: '\u{1F469}\u{200D}\u{1F52C}' },
];

// -- Component ---------------------------------------------------

export default function RolePlayRenderer({
    knowledgePoint,
    agentChannel,
    onInteractionEvent,
}: SkillRendererProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
    const [streamingContent, setStreamingContent] = useState('');

    // Role-play specific state
    const [characters, setCharacters] = useState<CharacterInfo[]>(DEFAULT_CHARACTERS);
    const [selectedCharacter, setSelectedCharacter] = useState<CharacterInfo | null>(null);
    const [scenario, setScenario] = useState<ScenarioInfo | null>(null);
    const [showSummary, setShowSummary] = useState(false);
    const [summaryContent, setSummaryContent] = useState('');
    const [isActive, setIsActive] = useState(true);

    const messagesEndRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLTextAreaElement>(null);

    // -- Modal a11y for summary overlay --------------------------
    const closeSummary = useCallback(() => setShowSummary(false), []);
    const summaryModalRef = useModalA11y(showSummary, closeSummary);

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
                        setThinkingStatus(event.payload?.status || '角色思考中...');
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
                                    id: `coach-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                                    role: 'coach',
                                    content: prev,
                                    timestamp: Date.now(),
                                    characterName: selectedCharacter?.name,
                                    tone: event.payload?.tone,
                                }]);
                            }
                            return '';
                        });
                        // Update scenario info if provided
                        if (event.payload?.scenario) {
                            setScenario(event.payload.scenario);
                        }
                        inputRef.current?.focus();
                        break;
                    }
                    case 'roleplay_character': {
                        // Backend sends available characters for the session
                        if (event.payload?.characters) {
                            setCharacters(event.payload.characters);
                        }
                        if (event.payload?.scenario) {
                            setScenario(event.payload.scenario);
                        }
                        break;
                    }
                    case 'roleplay_summary': {
                        // Learning summary when exiting role-play
                        setSummaryContent(event.payload?.summary || '');
                        setShowSummary(true);
                        setIsActive(false);
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
                            id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
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
    }, [agentChannel, selectedCharacter]);

    // -- Character selection -------------------------------------

    const handleSelectCharacter = useCallback((char: CharacterInfo) => {
        setSelectedCharacter(char);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: `[选择角色] ${char.name} (${char.role})`, character: char.name },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        onInteractionEvent({
            type: 'character_selected',
            payload: { skillId: 'general_review_roleplay', character: char.name },
            timestamp: Date.now(),
        });

        setSending(true);
    }, [agentChannel, onInteractionEvent]);

    // -- Exit role-play ------------------------------------------

    const handleExit = useCallback(() => {
        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: '[退出角色扮演]', action: 'exit_roleplay' },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        onInteractionEvent({
            type: 'roleplay_exit',
            payload: { skillId: 'general_review_roleplay' },
            timestamp: Date.now(),
        });

        setSending(true);
    }, [agentChannel, onInteractionEvent]);

    // -- Send message --------------------------------------------

    const handleSend = useCallback(() => {
        const text = input.trim();
        if (!text || sending || !isActive) return;

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
            payload: { skillId: 'general_review_roleplay', length: text.length },
            timestamp: Date.now(),
        });

        setInput('');
        setSending(true);
        setStreamingContent('');

        if (inputRef.current) {
            inputRef.current.style.height = 'auto';
        }
    }, [input, sending, isActive, agentChannel, onInteractionEvent]);

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

    // -- Render --------------------------------------------------

    // Character selection phase (no character chosen yet)
    if (!selectedCharacter) {
        return (
            <div className={styles.container}>
                {knowledgePoint.title && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8, padding: '8px 0', marginBottom: 4 }}>
                        <span style={{
                            fontSize: 11, fontWeight: 600, padding: '2px 10px', borderRadius: 100,
                            background: 'rgba(9, 132, 227, 0.1)', color: '#0984e3',
                            border: '1px solid rgba(9, 132, 227, 0.15)',
                        }}>角色扮演</span>
                        <span style={{ fontSize: 13, fontWeight: 600, color: 'var(--text-secondary)' }}>
                            {knowledgePoint.title}
                        </span>
                    </div>
                )}
                <div className={styles.characterSelector}>
                    <div className={styles.characterSelectorLabel}>选择你的对话角色</div>
                    <div className={styles.characterGrid}>
                        {characters.map(char => (
                            <div
                                key={char.name}
                                className={styles.characterCard}
                                {...cardA11yProps}
                                onClick={() => handleSelectCharacter(char)}
                            >
                                <div className={styles.characterAvatar}>{char.avatar}</div>
                                <div className={styles.characterName}>{char.name}</div>
                                <div className={styles.characterRole}>{char.role}</div>
                            </div>
                        ))}
                    </div>
                </div>
                <div className={styles.emptyHint}>
                    选择一个角色开始沉浸式学习对话
                </div>
            </div>
        );
    }

    // Active role-play session
    return (
        <div className={styles.container}>
            {/* Scenario panel */}
            {scenario && (
                <div className={styles.scenarioPanel}>
                    <div className={styles.scenarioHeader}>
                        <span className={styles.scenarioLabel}>场景</span>
                        {scenario.maxSwitches > 0 && (
                            <span className={styles.scenarioSwitch}>
                                场景切换: <span className={styles.scenarioSwitchCount}>{scenario.switches}</span>/{scenario.maxSwitches}
                            </span>
                        )}
                    </div>
                    <div className={styles.scenarioTitle}>
                        与{selectedCharacter.name}对话 — {knowledgePoint.title}
                    </div>
                    <div className={styles.scenarioDesc}>{scenario.description}</div>
                    {scenario.objectives.length > 0 && (
                        <div className={styles.scenarioObjectives}>
                            <div className={styles.objectivesLabel}>学习目标</div>
                            {scenario.objectives.map((obj, i) => (
                                <div key={i} className={styles.objectiveItem}>
                                    <span className={styles.objectiveBullet}>&#x2022;</span>
                                    <span>{obj}</span>
                                </div>
                            ))}
                        </div>
                    )}
                </div>
            )}

            {/* Messages */}
            <div className={styles.messagesArea}>
                {messages.length === 0 && !thinkingStatus && (
                    <div className={styles.emptyHint}>
                        {selectedCharacter.name}正在准备对话...
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
                                <span className={styles.avatarIcon}>
                                    {msg.role === 'student' ? '\u{1F464}' : selectedCharacter.avatar}
                                </span>
                                <div className={styles.characterLabel}>
                                    <span className={styles.characterNameLabel}>
                                        {msg.role === 'student' ? '我' : (msg.characterName || selectedCharacter.name)}
                                    </span>
                                    {msg.tone && (
                                        <span className={styles.toneIndicator}>{msg.tone}</span>
                                    )}
                                </div>
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
                            <span className={styles.avatarIcon}>{selectedCharacter.avatar}</span>
                            <div className={styles.characterLabel}>
                                <span className={styles.characterNameLabel}>{selectedCharacter.name}</span>
                            </div>
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

            {/* Input */}
            <div className={styles.inputArea}>
                <textarea
                    ref={inputRef}
                    className={styles.chatInput}
                    value={input}
                    onChange={handleInputChange}
                    onKeyDown={handleKeyDown}
                    placeholder={
                        !isActive ? '角色扮演已结束' :
                        sending ? `${selectedCharacter.name}正在思考...` :
                        `与${selectedCharacter.name}对话... (Enter 发送)`
                    }
                    disabled={sending || !isActive}
                    rows={1}
                />
                <div className={styles.inputActions}>
                    <button
                        className={`btn btn-primary ${styles.sendBtn}`}
                        onClick={handleSend}
                        disabled={!input.trim() || sending || !isActive}
                    >
                        发送
                    </button>
                    {isActive && (
                        <button
                            className={styles.exitBtn}
                            onClick={handleExit}
                            disabled={sending}
                        >
                            退出
                        </button>
                    )}
                </div>
            </div>

            {/* Learning summary overlay */}
            {showSummary && (
                <div className={styles.summaryOverlay} onClick={closeSummary}>
                    <div className={styles.summaryCard} ref={summaryModalRef} role="dialog" aria-modal="true" aria-labelledby="roleplay-summary-title" tabIndex={-1} onClick={e => e.stopPropagation()}>
                        <div className={styles.summaryIcon}>&#x1F4DA;</div>
                        <div id="roleplay-summary-title" className={styles.summaryTitle}>角色扮演学习总结</div>
                        <div className={styles.summaryContent}>
                            <MarkdownRenderer content={summaryContent} />
                        </div>
                        <button className={styles.summaryCloseBtn} onClick={closeSummary}>
                            关闭
                        </button>
                    </div>
                </div>
            )}
        </div>
    );
}
