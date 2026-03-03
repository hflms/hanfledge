'use client';

import MarkdownRenderer from '@/components/MarkdownRenderer';
import SurveyBlock from '@/components/SurveyBlock';

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
}

interface MessagePartText {
    type: 'text';
    content: string;
}

interface MessagePartSurvey {
    type: 'survey';
    payload: SurveyPayload;
}

type MessagePart = MessagePartText | MessagePartSurvey;

function parseSurveyParts(content: string): MessagePart[] {
    const parts: MessagePart[] = [];
    const regex = /<survey>([\s\S]*?)<\/survey>/g;
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = regex.exec(content)) !== null) {
        if (match.index > lastIndex) {
            parts.push({ type: 'text', content: content.slice(lastIndex, match.index) });
        }

        try {
            const payload = JSON.parse(match[1]) as SurveyPayload;
            parts.push({ type: 'survey', payload });
        } catch {
            parts.push({ type: 'text', content: match[0] });
        }

        lastIndex = match.index + match[0].length;
    }

    if (lastIndex < content.length) {
        parts.push({ type: 'text', content: content.slice(lastIndex) });
    }

    return parts;
}

export default function StructuredMessage({ content, isStreaming = false }: StructuredMessageProps) {
    const parts = parseSurveyParts(content);

    return (
        <>
            {parts.map((part, index) => {
                if (part.type === 'survey') {
                    return <SurveyBlock key={`survey-${index}`} survey={part.payload} />;
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
        </>
    );
}
