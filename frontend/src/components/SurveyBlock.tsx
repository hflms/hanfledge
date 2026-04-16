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

function LikertQuestion({
    question,
    selection,
    scaleLabels,
    onSelect
}: {
    question: SurveyQuestion;
    selection?: string;
    scaleLabels: string[];
    onSelect: (index: number, label: string) => void;
}) {
    return (
        <div className={styles.likertRow}>
            {(question.scale_labels || scaleLabels).map((label, index) => (
                <button
                    key={index}
                    type="button"
                    className={styles.likertOption}
                    onClick={() => onSelect(index, label)}
                    aria-pressed={selection === String(index + 1)}
                >
                    {label}
                </button>
            ))}
        </div>
    );
}

function MultipleChoiceQuestion({
    question,
    selections = [],
    onSelect
}: {
    question: SurveyQuestion;
    selections?: string[];
    onSelect: (key: string) => void;
}) {
    if (!question.options) return null;
    return (
        <ul className={styles.optionList}>
            {question.options.map(option => (
                <li key={option.key} className={styles.optionItem}>
                    <button
                        type="button"
                        className={styles.optionItemBtn}
                        onClick={() => onSelect(option.key)}
                        aria-pressed={selections.includes(option.key)}
                    >
                        <span className={styles.optionKey}>{option.key}.</span>
                        <span>{option.text}</span>
                    </button>
                </li>
            ))}
        </ul>
    );
}

function SingleChoiceQuestion({
    question,
    selection,
    onSelect
}: {
    question: SurveyQuestion;
    selection?: string;
    onSelect: (key: string) => void;
}) {
    if (!question.options) return null;
    return (
        <ul className={styles.optionList}>
            {question.options.map(option => (
                <li key={option.key} className={styles.optionItem}>
                    <button
                        type="button"
                        className={styles.optionItemBtn}
                        onClick={() => onSelect(option.key)}
                        aria-pressed={selection === option.key}
                    >
                        <span className={styles.optionKey}>{option.key}.</span>
                        <span>{option.text}</span>
                    </button>
                </li>
            ))}
        </ul>
    );
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

                        {question.type === 'single_choice' && (
                            <SingleChoiceQuestion
                                question={question}
                                selection={selections[question.id] as string}
                                onSelect={(key) => handleSingleChoice(question, key)}
                            />
                        )}

                        {question.type === 'multiple_choice' && (
                            <MultipleChoiceQuestion
                                question={question}
                                selections={selections[question.id] as string[]}
                                onSelect={(key) => handleMultipleChoice(question, key)}
                            />
                        )}

                        {question.type === 'likert_scale' && (
                            <LikertQuestion
                                question={question}
                                selection={selections[question.id] as string}
                                scaleLabels={scaleLabels}
                                onSelect={(index, label) => handleLikert(question, index, label)}
                            />
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
