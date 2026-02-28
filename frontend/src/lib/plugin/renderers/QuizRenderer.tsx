'use client';

/**
 * Quiz Generation Skill Renderer.
 *
 * Form-style UI with question cards (MCQ single/multiple + fill-blank),
 * per design.md §7.13 — Assessment sub-category.
 *
 * Phases: generating → answering → grading → reviewing
 * Supports <quiz>JSON</quiz> parsing from coach responses.
 */

import { useState, useRef, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './QuizRenderer.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
}

type QuizPhase = 'generating' | 'answering' | 'grading' | 'reviewing';

interface QuizOption {
    key: string;
    text: string;
}

interface QuizQuestion {
    id: number;
    type: 'mcq_single' | 'mcq_multiple' | 'fill_blank';
    stem: string;
    options?: QuizOption[];
    answer: string[];
    blanks?: string[];
    explanation: string;
}

interface QuizData {
    questions: QuizQuestion[];
}

interface GradedQuestion extends QuizQuestion {
    studentAnswer: string[];
    correct: boolean;
}

const PHASE_LABELS: Record<QuizPhase, string> = {
    generating: '出题中',
    answering: '作答中',
    grading: '批改中',
    reviewing: '查看结果',
};

// -- Quiz JSON Parser --------------------------------------------

function parseQuizFromContent(content: string): QuizData | null {
    const match = content.match(/<quiz>([\s\S]*?)<\/quiz>/);
    if (!match) return null;
    try {
        return JSON.parse(match[1]) as QuizData;
    } catch {
        return null;
    }
}

function stripQuizTag(content: string): string {
    return content.replace(/<quiz>[\s\S]*?<\/quiz>/, '').trim();
}

// -- Component ---------------------------------------------------

export default function QuizRenderer({
    agentChannel,
    onInteractionEvent,
}: SkillRendererProps) {
    const [messages, setMessages] = useState<ChatMessage[]>([]);
    const [input, setInput] = useState('');
    const [sending, setSending] = useState(false);
    const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
    const [streamingContent, setStreamingContent] = useState('');

    // Quiz-specific state
    const [phase, setPhase] = useState<QuizPhase>('generating');
    const [quizData, setQuizData] = useState<QuizData | null>(null);
    const [studentAnswers, setStudentAnswers] = useState<Record<number, string[]>>({});
    const [gradedResults, setGradedResults] = useState<GradedQuestion[]>([]);
    const [score, setScore] = useState<{ correct: number; total: number } | null>(null);

    const messagesEndRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLTextAreaElement>(null);

    // -- Scroll to bottom ----------------------------------------

    const scrollToBottom = useCallback(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, []);

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingContent, thinkingStatus, quizData, gradedResults, scrollToBottom]);

    // -- WebSocket message handling ------------------------------

    useEffect(() => {
        agentChannel.onMessage((data: string) => {
            try {
                const event = JSON.parse(data);
                switch (event.event) {
                    case 'agent_thinking': {
                        setThinkingStatus(event.payload?.status || '出题中...');
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
                                // Try to parse quiz data from the response
                                const parsed = parseQuizFromContent(prev);
                                if (parsed && parsed.questions.length > 0) {
                                    setQuizData(parsed);
                                    setStudentAnswers({});
                                    setGradedResults([]);
                                    setScore(null);
                                    setPhase('answering');

                                    // Store the intro text (before <quiz>) as a message
                                    const intro = stripQuizTag(prev);
                                    if (intro) {
                                        setMessages(msgs => [...msgs, {
                                            id: `coach-${Date.now()}`,
                                            role: 'coach',
                                            content: intro,
                                            timestamp: Date.now(),
                                        }]);
                                    }
                                } else {
                                    // Regular coach message (grading feedback, etc.)
                                    setMessages(msgs => [...msgs, {
                                        id: `coach-${Date.now()}`,
                                        role: 'coach',
                                        content: prev,
                                        timestamp: Date.now(),
                                    }]);

                                    // If we were in grading phase, move to reviewing
                                    if (phase === 'grading') {
                                        setPhase('reviewing');
                                    }
                                }
                            }
                            return '';
                        });
                        inputRef.current?.focus();
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
    }, [agentChannel, phase, onInteractionEvent]);

    // -- MCQ answer handling -------------------------------------

    const handleMCQSelect = useCallback((questionId: number, optionKey: string, isMultiple: boolean) => {
        setStudentAnswers(prev => {
            const current = prev[questionId] || [];
            if (isMultiple) {
                // Toggle selection for multiple choice
                const idx = current.indexOf(optionKey);
                if (idx >= 0) {
                    return { ...prev, [questionId]: current.filter(k => k !== optionKey) };
                }
                return { ...prev, [questionId]: [...current, optionKey] };
            }
            // Single choice: replace
            return { ...prev, [questionId]: [optionKey] };
        });
    }, []);

    // -- Fill-blank answer handling ------------------------------

    const handleBlankInput = useCallback((questionId: number, blankIndex: number, value: string) => {
        setStudentAnswers(prev => {
            const current = prev[questionId] || [];
            const updated = [...current];
            updated[blankIndex] = value;
            return { ...prev, [questionId]: updated };
        });
    }, []);

    // -- Submit answers -----------------------------------------

    const handleSubmitAnswers = useCallback(() => {
        if (!quizData) return;

        // Grade locally for immediate feedback
        const graded: GradedQuestion[] = quizData.questions.map(q => {
            const studentAns = studentAnswers[q.id] || [];
            let correct = false;

            if (q.type === 'fill_blank') {
                // Fill-blank: compare each blank (case-insensitive, trimmed)
                const expected = q.blanks || q.answer;
                correct = expected.length === studentAns.length &&
                    expected.every((exp, i) =>
                        studentAns[i]?.trim().toLowerCase() === exp.trim().toLowerCase()
                    );
            } else {
                // MCQ: compare sorted answer arrays
                const sortedExpected = [...q.answer].sort();
                const sortedStudent = [...studentAns].sort();
                correct = sortedExpected.length === sortedStudent.length &&
                    sortedExpected.every((v, i) => v === sortedStudent[i]);
            }

            return { ...q, studentAnswer: studentAns, correct };
        });

        setGradedResults(graded);
        const correctCount = graded.filter(g => g.correct).length;
        setScore({ correct: correctCount, total: graded.length });
        setPhase('grading');

        // Send answers to backend for full grading with explanations
        const answerText = quizData.questions.map(q => {
            const ans = studentAnswers[q.id] || [];
            if (q.type === 'fill_blank') {
                return `第${q.id}题: ${ans.join(', ') || '(未作答)'}`;
            }
            return `第${q.id}题: ${ans.join('') || '(未作答)'}`;
        }).join('\n');

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: `我的答案:\n${answerText}` },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        setSending(true);
        setStreamingContent('');

        onInteractionEvent({
            type: 'quiz_submitted',
            payload: {
                skillId: 'general_assessment_quiz',
                questionCount: graded.length,
                correctCount,
            },
            timestamp: Date.now(),
        });
    }, [quizData, studentAnswers, agentChannel, onInteractionEvent]);

    // -- Request more questions ----------------------------------

    const handleRequestMore = useCallback(() => {
        setPhase('generating');
        setQuizData(null);
        setGradedResults([]);
        setScore(null);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: '请再出一批题目' },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        setSending(true);
        setStreamingContent('');
    }, [agentChannel]);

    // -- Free-form message send ---------------------------------

    const handleSend = useCallback(() => {
        const text = input.trim();
        if (!text || sending) return;

        setMessages(prev => [...prev, {
            id: `student-${Date.now()}`,
            role: 'student',
            content: text,
            timestamp: Date.now(),
        }]);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        setInput('');
        setSending(true);
        setStreamingContent('');

        if (inputRef.current) {
            inputRef.current.style.height = 'auto';
        }
    }, [input, sending, agentChannel]);

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

    // -- Render question card -----------------------------------

    const renderQuestion = (q: QuizQuestion, graded?: GradedQuestion) => {
        const isGraded = !!graded;
        const selected = studentAnswers[q.id] || [];
        const isMultiple = q.type === 'mcq_multiple';

        return (
            <div key={q.id} className={`${styles.questionCard} ${isGraded ? (graded.correct ? styles.cardCorrect : styles.cardIncorrect) : ''}`}>
                <div className={styles.questionHeader}>
                    <span className={styles.questionNumber}>第 {q.id} 题</span>
                    <span className={styles.questionType}>
                        {q.type === 'mcq_single' ? '单选题' : q.type === 'mcq_multiple' ? '多选题' : '填空题'}
                    </span>
                    {isGraded && (
                        <span className={graded.correct ? styles.resultCorrect : styles.resultIncorrect}>
                            {graded.correct ? '正确' : '错误'}
                        </span>
                    )}
                </div>

                <div className={styles.questionStem}>{q.stem}</div>

                {/* MCQ options */}
                {q.options && (
                    <div className={styles.optionsList}>
                        {q.options.map(opt => {
                            const isSelected = selected.includes(opt.key);
                            const isCorrectAnswer = isGraded && q.answer.includes(opt.key);
                            let optionClass = styles.option;
                            if (isGraded) {
                                if (isCorrectAnswer) optionClass += ` ${styles.optionCorrect}`;
                                else if (isSelected) optionClass += ` ${styles.optionIncorrect}`;
                            } else if (isSelected) {
                                optionClass += ` ${styles.optionSelected}`;
                            }

                            return (
                                <button
                                    key={opt.key}
                                    className={optionClass}
                                    onClick={() => !isGraded && handleMCQSelect(q.id, opt.key, isMultiple)}
                                    disabled={isGraded || phase !== 'answering'}
                                >
                                    <span className={styles.optionKey}>{opt.key}</span>
                                    <span className={styles.optionText}>{opt.text}</span>
                                </button>
                            );
                        })}
                    </div>
                )}

                {/* Fill-blank inputs */}
                {q.type === 'fill_blank' && (
                    <div className={styles.blanksList}>
                        {(q.blanks || q.answer).map((_, idx) => (
                            <div key={idx} className={styles.blankRow}>
                                <span className={styles.blankLabel}>空 {idx + 1}:</span>
                                <input
                                    className={`${styles.blankInput} ${
                                        isGraded
                                            ? (graded.studentAnswer[idx]?.trim().toLowerCase() === (q.blanks || q.answer)[idx]?.trim().toLowerCase()
                                                ? styles.blankCorrect
                                                : styles.blankIncorrect)
                                            : ''
                                    }`}
                                    value={selected[idx] || ''}
                                    onChange={e => !isGraded && handleBlankInput(q.id, idx, e.target.value)}
                                    placeholder="输入答案"
                                    disabled={isGraded || phase !== 'answering'}
                                />
                                {isGraded && !graded.correct && (
                                    <span className={styles.correctAnswer}>
                                        正确答案: {(q.blanks || q.answer)[idx]}
                                    </span>
                                )}
                            </div>
                        ))}
                    </div>
                )}

                {/* Explanation (shown after grading) */}
                {isGraded && q.explanation && (
                    <div className={styles.explanation}>
                        <div className={styles.explanationLabel}>解析</div>
                        <MarkdownRenderer content={q.explanation} />
                    </div>
                )}
            </div>
        );
    };

    // -- Render thinking indicator --------------------------------

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

    // -- Check if all questions answered -------------------------

    const allAnswered = quizData?.questions.every(q => {
        const ans = studentAnswers[q.id] || [];
        if (q.type === 'fill_blank') {
            return (q.blanks || q.answer).every((_, idx) => ans[idx]?.trim());
        }
        return ans.length > 0;
    }) ?? false;

    // -- Main render ---------------------------------------------

    return (
        <div className={styles.quizContainer}>
            {/* Phase indicator */}
            <div className={styles.phaseBar}>
                {(Object.entries(PHASE_LABELS) as [QuizPhase, string][]).map(([p, label]) => (
                    <div
                        key={p}
                        className={`${styles.phaseStep} ${p === phase ? styles.phaseActive : ''} ${
                            (Object.keys(PHASE_LABELS).indexOf(p) < Object.keys(PHASE_LABELS).indexOf(phase))
                                ? styles.phaseCompleted : ''
                        }`}
                    >
                        {label}
                    </div>
                ))}
            </div>

            {/* Messages area */}
            <div className={styles.messagesArea}>
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

                {renderThinking()}

                {/* Quiz cards */}
                {quizData && phase === 'answering' && (
                    <div className={styles.quizCards}>
                        {quizData.questions.map(q => renderQuestion(q))}
                        <button
                            className={styles.submitBtn}
                            onClick={handleSubmitAnswers}
                            disabled={!allAnswered || sending}
                        >
                            {allAnswered ? '提交答案' : '请完成所有题目'}
                        </button>
                    </div>
                )}

                {/* Graded results */}
                {gradedResults.length > 0 && (phase === 'grading' || phase === 'reviewing') && (
                    <div className={styles.quizCards}>
                        {score && (
                            <div className={styles.scoreCard}>
                                <div className={styles.scoreNumber}>
                                    {score.correct} / {score.total}
                                </div>
                                <div className={styles.scoreLabel}>
                                    正确率: {Math.round((score.correct / score.total) * 100)}%
                                </div>
                            </div>
                        )}
                        {gradedResults.map(g => renderQuestion(g, g))}
                        {phase === 'reviewing' && (
                            <button
                                className={styles.moreBtn}
                                onClick={handleRequestMore}
                                disabled={sending}
                            >
                                继续出题
                            </button>
                        )}
                    </div>
                )}

                <div ref={messagesEndRef} />
            </div>

            {/* Input area (for asking questions about quiz) */}
            <div className={styles.inputArea}>
                <textarea
                    ref={inputRef}
                    className={styles.chatInput}
                    value={input}
                    onChange={handleInputChange}
                    onKeyDown={handleKeyDown}
                    placeholder={
                        sending
                            ? 'AI 正在处理...'
                            : phase === 'answering'
                            ? '有疑问可以在这里提问 (Enter 发送)'
                            : '输入你的想法或问题... (Enter 发送)'
                    }
                    disabled={sending}
                    rows={1}
                />
                <button
                    className={styles.sendBtn}
                    onClick={handleSend}
                    disabled={!input.trim() || sending}
                >
                    发送
                </button>
            </div>
        </div>
    );
}
