'use client';

import MarkdownRenderer from '@/components/MarkdownRenderer';
import SurveyBlock from '@/components/SurveyBlock';
import InlineQuizBlock, { QuizPayload } from '@/components/InlineQuizBlock';
import styles from './StructuredMessage.module.css';

interface SurveyOption {
    key: string;
    text: string;
}

interface SurveyQuestion {
    id: number;
    type: 'single_choice' | 'multiple_choice' | 'likert_scale' | 'open_ended';
    stem: string;
    options?: SurveyOption[];
    scale_labels?: string[];
}

interface SurveyPayload {
    dimension: string;
    dimension_label: string;
    questions: SurveyQuestion[];
}

interface StructuredMessageProps {
    content: string;
    isStreaming?: boolean;
    onSurveySelect?: (text: string) => void;
    onQuickReply?: (text: string) => void;
}

interface MessagePartText {
    type: 'text';
    content: string;
}

interface MessagePartSurvey {
    type: 'survey';
    payload: SurveyPayload;
}

interface MessagePartQuiz {
    type: 'quiz';
    payload: QuizPayload;
}

type MessagePart = MessagePartText | MessagePartSurvey | MessagePartQuiz;

function parseStructuredParts(content: string): { parts: MessagePart[], suggestions: string[] } {
    let suggestions: string[] = [];
    let cleanContent = content;
    
    // Extract suggestions from <suggestions> tags
    const suggRegex = /<suggestions>([\s\S]*?)<\/suggestions>/g;
    let suggMatch: RegExpExecArray | null;
    while ((suggMatch = suggRegex.exec(content)) !== null) {
        try {
            const parsed = JSON.parse(suggMatch[1]);
            if (Array.isArray(parsed)) {
                suggestions = parsed.filter(item => typeof item === 'string');
            }
        } catch {
            // Ignore parse errors
        }
        cleanContent = cleanContent.replace(suggMatch[0], '');
    }

    const parts: MessagePart[] = [];
    
    // Process <survey> and <quiz> sequentially
    const componentRegex = /<(survey|quiz)>([\s\S]*?)<\/\1>/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = componentRegex.exec(cleanContent)) !== null) {
        if (match.index > lastIndex) {
            parts.push({ type: 'text', content: cleanContent.slice(lastIndex, match.index) });
        }

        const tag = match[1];
        try {
            const payload = JSON.parse(match[2]);
            if (tag === 'survey') {
                parts.push({ type: 'survey', payload: payload as SurveyPayload });
            } else if (tag === 'quiz') {
                parts.push({ type: 'quiz', payload: payload as QuizPayload });
            }
        } catch {
            parts.push({ type: 'text', content: match[0] });
        }

        lastIndex = match.index + match[0].length;
    }

    if (lastIndex < cleanContent.length) {
        parts.push({ type: 'text', content: cleanContent.slice(lastIndex) });
    }

    return { parts, suggestions };
}

export default function StructuredMessage({ content, isStreaming = false, onSurveySelect, onQuickReply }: StructuredMessageProps) {
    const { parts, suggestions } = parseStructuredParts(content);

    return (
        <>
            {parts.map((part, index) => {
                if (part.type === 'survey') {
                    return <SurveyBlock key={`survey-${index}`} survey={part.payload} onSelect={onSurveySelect} />;
                }
                if (part.type === 'quiz') {
                    return <InlineQuizBlock key={`quiz-${index}`} quiz={part.payload} onComplete={(answers) => {
                        // Create a summary text of the student's answers
                        const summary = part.payload.questions.map(q => {
                            const ans = answers[q.id] || [];
                            return `第${q.id}题我的答案是: ${ans.join(',')}`;
                        }).join('; ');
                        onQuickReply?.(summary);
                    }} />;
                }
                if (!part.content.trim()) {
                    return null;
                }
                return (
                    <MarkdownRenderer
                        key={`text-${index}`}
                        content={part.content}
                        isStreaming={isStreaming && index === parts.length - 1}
                    />
                );
            })}
            
            {!isStreaming && suggestions.length > 0 && (
                <div className={styles.suggestionsContainer}>
                    {suggestions.map((suggestion, index) => (
                        <button 
                            key={`suggestion-${index}`} 
                            className={styles.suggestionPill}
                            onClick={() => onQuickReply?.(suggestion)}
                        >
                            {suggestion}
                        </button>
                    ))}
                </div>
            )}
        </>
    );
}
