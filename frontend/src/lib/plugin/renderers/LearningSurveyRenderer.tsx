'use client';

/**
 * Learning Survey Skill Renderer.
 *
 * Renders a structured questionnaire UI with multiple question types
 * (single_choice, multiple_choice, likert_scale, open_ended).
 * Displays survey progress, learning profile, and learning plan results.
 *
 * Phases: welcome → surveying → analyzing → reporting → planning
 */

import { useState, useRef, useCallback, useEffect } from 'react';
import dynamic from 'next/dynamic';
import ChatInputArea from '@/components/ChatInputArea';
import type { SkillRendererProps } from '@/lib/plugin/types';
import styles from './LearningSurveyRenderer.module.css';

const StructuredMessage = dynamic(() => import('@/components/StructuredMessage'));

// -- Types -------------------------------------------------------

interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system';
    content: string;
    timestamp: number;
}

interface SurveyQuestion {
    id: number;
    type: 'single_choice' | 'multiple_choice' | 'likert_scale' | 'open_ended';
    stem: string;
    options?: { key: string; text: string }[];
    scale_labels?: string[];
}

interface SurveyBatch {
    dimension: string;
    dimension_label: string;
    questions: SurveyQuestion[];
}

interface SurveyProgress {
    completedDims: number;
    totalDims: number;
    phase: string;
}

// -- Component ---------------------------------------------------

export default function LearningSurveyRenderer({
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

    // Survey-specific state
    const [currentSurvey, setCurrentSurvey] = useState<SurveyBatch | null>(null);
    const [answers, setAnswers] = useState<Record<number, string | string[]>>({});
    const [progress, setProgress] = useState<SurveyProgress>({
        completedDims: 0,
        totalDims: 6,
        phase: 'welcome',
    });
    const [profileReady, setProfileReady] = useState(false);
    const [planReady, setPlanReady] = useState(false);

    const messagesEndRef = useRef<HTMLDivElement>(null);

    // -- Scroll to bottom ----------------------------------------

    const scrollToBottom = useCallback(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, []);

    useEffect(() => {
        scrollToBottom();
    }, [messages, streamingContent, thinkingStatus, currentSurvey, scrollToBottom]);

    // -- Parse survey from coach response ------------------------

    const parseSurveyFromContent = useCallback((content: string): SurveyBatch | null => {
        const match = content.match(/<survey>([\s\S]*?)<\/survey>/);
        if (!match) return null;
        try {
            return JSON.parse(match[1]) as SurveyBatch;
        } catch {
            return null;
        }
    }, []);

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
                            action: string;
                            data: Record<string, unknown>;
                        };

                        if (payload.action === 'survey_questions') {
                            setProgress(prev => ({
                                ...prev,
                                completedDims: (payload.data.completed_dims as number) ?? prev.completedDims,
                                totalDims: (payload.data.total_dims as number) ?? prev.totalDims,
                                phase: 'surveying',
                            }));
                        } else if (payload.action === 'learning_profile') {
                            setProfileReady(true);
                            setProgress(prev => ({ ...prev, phase: 'reporting' }));
                        } else if (payload.action === 'learning_plan') {
                            setPlanReady(true);
                            setProgress(prev => ({ ...prev, phase: 'planning' }));
                        } else if (payload.action === 'survey_analysis') {
                            setProgress(prev => ({ ...prev, phase: 'analyzing' }));
                        } else if (payload.action === 'scaffold_change') {
                            const d = payload.data as {
                                new_level: 'high' | 'medium' | 'low';
                                mastery: number;
                                direction: string;
                            };
                            const direction = d.direction === 'fade' ? '降低' : '增强';
                            const labels = { high: '高支架', medium: '中支架', low: '低支架' };
                            setMessages(prev => [...prev, {
                                id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                                role: 'system',
                                content: `支架已${direction}至 ${labels[d.new_level]} (掌握度: ${(d.mastery * 100).toFixed(0)}%)`,
                                timestamp: Date.now(),
                            }]);
                        }
                        break;
                    }
                    case 'turn_complete': {
                        setThinkingStatus(null);
                        setSending(false);
                        setStreamingContent(prev => {
                            if (prev) {
                                // Parse survey questions from response
                                const survey = parseSurveyFromContent(prev);
                                if (survey) {
                                    setCurrentSurvey(survey);
                                    setAnswers({});
                                }

                                setMessages(msgs => [...msgs, {
                                    id: `coach-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
                                    role: 'coach',
                                    content: prev,
                                    timestamp: Date.now(),
                                }]);
                            }
                            return '';
                        });
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
    }, [agentChannel, parseSurveyFromContent]);

    // -- Handle survey answer changes ----------------------------

    const handleSingleChoice = useCallback((questionId: number, key: string) => {
        setAnswers(prev => ({ ...prev, [questionId]: key }));
    }, []);

    const handleMultipleChoice = useCallback((questionId: number, key: string) => {
        setAnswers(prev => {
            const current = (prev[questionId] as string[]) || [];
            const updated = current.includes(key)
                ? current.filter(k => k !== key)
                : [...current, key];
            return { ...prev, [questionId]: updated };
        });
    }, []);

    const handleLikertScale = useCallback((questionId: number, value: string) => {
        setAnswers(prev => ({ ...prev, [questionId]: value }));
    }, []);

    const handleOpenEnded = useCallback((questionId: number, text: string) => {
        setAnswers(prev => ({ ...prev, [questionId]: text }));
    }, []);

    // -- Submit survey answers -----------------------------------

    const handleSubmitSurvey = useCallback(() => {
        if (!currentSurvey || sending) return;

        // Format answers into a readable string
        const answerLines = currentSurvey.questions.map(q => {
            const answer = answers[q.id];
            if (!answer) return `${q.id}. (未回答)`;

            switch (q.type) {
                case 'single_choice':
                    return `${q.id}. ${answer}`;
                case 'multiple_choice':
                    return `${q.id}. ${(answer as string[]).join(', ')}`;
                case 'likert_scale':
                    return `${q.id}. ${answer}`;
                case 'open_ended':
                    return `${q.id}. ${answer}`;
                default:
                    return `${q.id}. ${answer}`;
            }
        });

        const submissionText = `【${currentSurvey.dimension_label}】问卷回答：\n${answerLines.join('\n')}`;

        setMessages(prev => [...prev, {
            id: `student-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
            role: 'student',
            content: submissionText,
            timestamp: Date.now(),
        }]);

        agentChannel.send(JSON.stringify({
            event: 'user_message',
            payload: { text: submissionText },
            timestamp: Math.floor(Date.now() / 1000),
        }));

        onInteractionEvent({
            type: 'survey_submitted',
            payload: {
                skillId: 'general_diagnosis_survey',
                dimension: currentSurvey.dimension,
                questionCount: currentSurvey.questions.length,
                answeredCount: Object.keys(answers).length,
            },
            timestamp: Date.now(),
        });

        setCurrentSurvey(null);
        setAnswers({});
        setSending(true);
        setStreamingContent('');
    }, [currentSurvey, answers, sending, agentChannel, onInteractionEvent]);

    // -- Send free-text message ----------------------------------

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
            payload: { skillId: 'general_diagnosis_survey', length: text.length },
            timestamp: Date.now(),
        });

        setInput('');
        setSending(true);
        setStreamingContent('');
    }, [input, sending, agentChannel, onInteractionEvent]);

    

    // -- Strip <survey> / <survey_profile> / <learning_plan> tags from display content --

    const stripStructuredTags = (content: string): string => {
        return content
            .replace(/<survey>[\s\S]*?<\/survey>/g, '')
            .replace(/<survey_profile>[\s\S]*?<\/survey_profile>/g, '')
            .replace(/<learning_plan>[\s\S]*?<\/learning_plan>/g, '')
            .trim();
    };

    // -- Render Survey Question ----------------------------------

    const renderQuestion = (q: SurveyQuestion) => {
        switch (q.type) {
            case 'single_choice':
                return (
                    <div key={q.id} className={styles.questionCard}>
                        <div className={styles.questionStem}>{q.id}. {q.stem}</div>
                        <div className={styles.optionsList}>
                            {q.options?.map(opt => (
                                <label
                                    key={opt.key}
                                    className={`${styles.optionItem} ${answers[q.id] === opt.key ? styles.optionSelected : ''}`}
                                    onClick={() => handleSingleChoice(q.id, opt.key)}
                                >
                                    <span className={styles.optionRadio}>
                                        {answers[q.id] === opt.key ? '●' : '○'}
                                    </span>
                                    <span className={styles.optionKey}>{opt.key}.</span>
                                    <span>{opt.text}</span>
                                </label>
                            ))}
                        </div>
                    </div>
                );

            case 'multiple_choice':
                return (
                    <div key={q.id} className={styles.questionCard}>
                        <div className={styles.questionStem}>{q.id}. {q.stem} <span className={styles.multiHint}>(多选)</span></div>
                        <div className={styles.optionsList}>
                            {q.options?.map(opt => {
                                const selected = ((answers[q.id] as string[]) || []).includes(opt.key);
                                return (
                                    <label
                                        key={opt.key}
                                        className={`${styles.optionItem} ${selected ? styles.optionSelected : ''}`}
                                        onClick={() => handleMultipleChoice(q.id, opt.key)}
                                    >
                                        <span className={styles.optionCheckbox}>
                                            {selected ? '☑' : '☐'}
                                        </span>
                                        <span className={styles.optionKey}>{opt.key}.</span>
                                        <span>{opt.text}</span>
                                    </label>
                                );
                            })}
                        </div>
                    </div>
                );

            case 'likert_scale': {
                const scaleLabels = q.scale_labels || ['完全不同意', '不太同意', '一般', '比较同意', '非常同意'];
                return (
                    <div key={q.id} className={styles.questionCard}>
                        <div className={styles.questionStem}>{q.id}. {q.stem}</div>
                        <div className={styles.likertScale}>
                            {scaleLabels.map((label, i) => (
                                <button
                                    key={i}
                                    className={`${styles.likertOption} ${answers[q.id] === String(i + 1) ? styles.likertSelected : ''}`}
                                    onClick={() => handleLikertScale(q.id, String(i + 1))}
                                >
                                    <span className={styles.likertDot}>
                                        {answers[q.id] === String(i + 1) ? '●' : '○'}
                                    </span>
                                    <span className={styles.likertLabel}>{label}</span>
                                </button>
                            ))}
                        </div>
                    </div>
                );
            }

            case 'open_ended':
                return (
                    <div key={q.id} className={styles.questionCard}>
                        <div className={styles.questionStem}>{q.id}. {q.stem}</div>
                        <textarea
                            className={styles.openEndedInput}
                            value={(answers[q.id] as string) || ''}
                            onChange={e => handleOpenEnded(q.id, e.target.value)}
                            placeholder="请输入你的回答..."
                            rows={3}
                        />
                    </div>
                );

            default:
                return null;
        }
    };

    // -- Render Progress Bar -------------------------------------

    const renderProgressBar = () => {
        const pct = progress.totalDims > 0
            ? (progress.completedDims / progress.totalDims) * 100
            : 0;

        const phaseLabels: Record<string, string> = {
            welcome: '欢迎',
            surveying: '问卷进行中',
            analyzing: '分析中',
            reporting: '生成画像',
            planning: '制定方案',
        };

        return (
            <div className={styles.progressContainer}>
                <div className={styles.progressHeader}>
                    <span className={styles.progressLabel}>
                        诊断进度: {progress.completedDims} / {progress.totalDims} 维度
                    </span>
                    <span className={styles.progressPhase}>
                        {phaseLabels[progress.phase] || progress.phase}
                    </span>
                </div>
                <div className={styles.progressTrack}>
                    <div className={styles.progressFill} style={{ width: `${pct}%` }} />
                </div>
            </div>
        );
    };

    // -- Render --------------------------------------------------

    const allAnswered = currentSurvey
        ? currentSurvey.questions.every(q => {
            const a = answers[q.id];
            if (!a) return false;
            if (Array.isArray(a)) return a.length > 0;
            return String(a).trim().length > 0;
        })
        : false;

    return (
        <div className={styles.container}>
            {/* KP context header */}
            {knowledgePoint.title && (
                <div className={styles.kpHeader}>
                    <span className={styles.kpBadge}>学情诊断</span>
                    <span className={styles.kpTitle}>{knowledgePoint.title}</span>
                    {scaffoldingLevel && (
                        <span className={styles.scaffoldBadge}>
                            {scaffoldingLevel === 'high' ? '高支架' : scaffoldingLevel === 'medium' ? '中支架' : '低支架'}
                        </span>
                    )}
                </div>
            )}

            {/* Progress Bar */}
            {progress.phase !== 'welcome' && renderProgressBar()}

            {/* Status badges */}
            <div className={styles.statusRow}>
                {profileReady && <span className={styles.statusBadge}>学习画像已生成</span>}
                {planReady && <span className={styles.statusBadge}>学习方案已生成</span>}
            </div>

            {/* Messages */}
            <div className={styles.messagesArea}>
                {messages.length === 0 && !thinkingStatus && !currentSurvey && (
                    <div className={styles.emptyHint}>
                        发送消息开始学情问卷诊断
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
                                    {msg.role === 'student' ? '我' : '诊断助手'}
                                </span>
                            </div>
                        )}
                        <div className={styles.messageContent}>
                            {msg.role === 'coach' ? (
                                <StructuredMessage content={stripStructuredTags(msg.content)} />
                            ) : msg.content}
                        </div>
                    </div>
                ))}

                {/* Streaming content */}
                {streamingContent && (
                    <div className={`${styles.messageBubble} ${styles.messageCoach}`}>
                        <div className={styles.messageHeader}>
                            <span className={`${styles.roleIcon} ${styles.roleCoach}`}>AI</span>
                            <span className={styles.roleLabel}>诊断助手</span>
                        </div>
                        <div className={styles.messageContent}>
                            <StructuredMessage content={stripStructuredTags(streamingContent)} isStreaming />
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

            {/* Survey Question Panel */}
            {currentSurvey && (
                <div className={styles.surveyPanel}>
                    <div className={styles.surveyPanelHeader}>
                        <span className={styles.dimensionBadge}>
                            {currentSurvey.dimension_label}
                        </span>
                        <span className={styles.questionCount}>
                            {currentSurvey.questions.length} 道题
                        </span>
                    </div>
                    <div className={styles.surveyQuestions}>
                        {currentSurvey.questions.map(q => renderQuestion(q))}
                    </div>
                    <div className={styles.surveyActions}>
                        <button
                            className={`btn btn-primary ${styles.submitBtn}`}
                            onClick={handleSubmitSurvey}
                            disabled={!allAnswered || sending}
                        >
                            {allAnswered ? '提交回答' : `还有 ${currentSurvey.questions.length - Object.keys(answers).length} 题未答`}
                        </button>
                    </div>
                </div>
            )}

            {/* Input */}
            <ChatInputArea
                input={input}
                setInput={setInput}
                sending={sending}
                onSend={() => handleSend()}
                placeholder={sending ? '诊断助手正在思考...' : currentSurvey ? '也可以直接输入文字回答...' : '输入消息开始诊断...'}
            />
        </div>
    );
}