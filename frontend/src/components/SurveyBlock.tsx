'use client';

import { useMemo, useState } from 'react';
import styles from './SurveyBlock.module.css';

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

interface SurveyBlockProps {
    survey: SurveyPayload;
    onSelect?: (text: string) => void;
}

type SelectionState = Record<number, string | string[]>;

function formatSelection(question: SurveyQuestion, selection: string | string[], label?: string) {
    const base = `Q${question.id}: `;
    if (question.type === 'multiple_choice') {
        const values = Array.isArray(selection) ? selection : [selection];
        return base + values.join(', ');
    }
    if (question.type === 'likert_scale') {
        return base + (label ? `${selection}(${label})` : String(selection));
    }
    return base + String(selection);
}

export default function SurveyBlock({ survey, onSelect }: SurveyBlockProps) {
    const [selections, setSelections] = useState<SelectionState>({});
    const scaleLabels = useMemo(
        () => ['完全不同意', '不太同意', '一般', '比较同意', '非常同意'],
        []
    );

    const handleSingleChoice = (question: SurveyQuestion, key: string) => {
        if (selections[question.id] === key) return;
        const nextSelections = { ...selections, [question.id]: key };
        setSelections(nextSelections);
        onSelect?.(formatSelection(question, key));
    };

    const handleMultipleChoice = (question: SurveyQuestion, key: string) => {
        const current = selections[question.id];
        const next = Array.isArray(current) ? [...current] : [];
        const index = next.indexOf(key);
        if (index >= 0) {
            next.splice(index, 1);
        } else {
            next.push(key);
        }
        setSelections({ ...selections, [question.id]: next });
        onSelect?.(formatSelection(question, next));
    };

    const handleLikert = (question: SurveyQuestion, index: number, label: string) => {
        const value = String(index + 1);
        if (selections[question.id] === value) return;
        setSelections({ ...selections, [question.id]: value });
        onSelect?.(formatSelection(question, value, label));
    };

    return (
        <div className={styles.surveyCard}>
            <div className={styles.surveyHeader}>
                <span className={styles.surveyBadge}>问卷</span>
                <span className={styles.surveyTitle}>{survey.dimension_label || survey.dimension}</span>
            </div>

            <div className={styles.questionList}>
                {survey.questions.map(question => (
                    <div key={question.id} className={styles.questionItem}>
                        <div className={styles.questionStem}>
                            {question.id}. {question.stem}
                        </div>

                        {question.type === 'single_choice' && question.options && (
                            <ul className={styles.optionList}>
                                {question.options.map(option => (
                                    <li
                                        key={option.key}
                                        className={styles.optionItem}
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => handleSingleChoice(question, option.key)}
                                        onKeyDown={(e) => {
                                            if (e.key === 'Enter' || e.key === ' ') {
                                                e.preventDefault();
                                                handleSingleChoice(question, option.key);
                                            }
                                        }}
                                        aria-pressed={selections[question.id] === option.key}
                                    >
                                        <span className={styles.optionKey}>{option.key}.</span>
                                        <span>{option.text}</span>
                                    </li>
                                ))}
                            </ul>
                        )}

                        {question.type === 'multiple_choice' && question.options && (
                            <ul className={styles.optionList}>
                                {question.options.map(option => (
                                    <li
                                        key={option.key}
                                        className={styles.optionItem}
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => handleMultipleChoice(question, option.key)}
                                        onKeyDown={(e) => {
                                            if (e.key === 'Enter' || e.key === ' ') {
                                                e.preventDefault();
                                                handleMultipleChoice(question, option.key);
                                            }
                                        }}
                                        aria-pressed={Array.isArray(selections[question.id]) && (selections[question.id] as string[]).includes(option.key)}
                                    >
                                        <span className={styles.optionKey}>{option.key}.</span>
                                        <span>{option.text}</span>
                                    </li>
                                ))}
                            </ul>
                        )}

                        {question.type === 'likert_scale' && (
                            <div className={styles.likertRow}>
                                {(question.scale_labels || scaleLabels).map((label, index) => (
                                    <span
                                        key={index}
                                        className={styles.likertOption}
                                        role="button"
                                        tabIndex={0}
                                        onClick={() => handleLikert(question, index, label)}
                                        onKeyDown={(e) => {
                                            if (e.key === 'Enter' || e.key === ' ') {
                                                e.preventDefault();
                                                handleLikert(question, index, label);
                                            }
                                        }}
                                        aria-pressed={selections[question.id] === String(index + 1)}
                                    >
                                        {label}
                                    </span>
                                ))}
                            </div>
                        )}

                        {question.type === 'open_ended' && (
                            <div className={styles.openEndedHint}>开放回答题</div>
                        )}
                    </div>
                ))}
            </div>
        </div>
    );
}
