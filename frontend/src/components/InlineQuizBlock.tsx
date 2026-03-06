'use client';

import React, { useState } from 'react';
import styles from './InlineQuizBlock.module.css';

interface QuizOption {
    key: string;
    text: string;
}

export interface QuizQuestion {
    id: number;
    type: 'mcq_single' | 'mcq_multiple' | 'fill_blank';
    stem: string;
    options?: QuizOption[];
    answer: string[];
    explanation?: string;
}

export interface QuizPayload {
    questions: QuizQuestion[];
}

interface Props {
    quiz: QuizPayload;
    onComplete?: (answers: Record<number, string[]>) => void;
}

export default function InlineQuizBlock({ quiz, onComplete }: Props) {
    const [answers, setAnswers] = useState<Record<number, string[]>>({});
    const [submitted, setSubmitted] = useState(false);

    const handleSelect = (qId: number, key: string, isMulti: boolean) => {
        if (submitted) return;
        setAnswers(prev => {
            const current = prev[qId] || [];
            if (isMulti) {
                return { ...prev, [qId]: current.includes(key) ? current.filter(k => k !== key) : [...current, key] };
            } else {
                return { ...prev, [qId]: [key] };
            }
        });
    };

    const handleSubmit = () => {
        setSubmitted(true);
        onComplete?.(answers);
    };

    return (
        <div className={styles.quizContainer}>
            {quiz.questions.map(q => {
                const selected = answers[q.id] || [];
                const isCorrect = submitted && q.answer.every(a => selected.includes(a)) && selected.every(s => q.answer.includes(s));
                
                return (
                    <div key={q.id} className={`${styles.questionCard} ${submitted ? (isCorrect ? styles.correct : styles.incorrect) : ''}`}>
                        <div className={styles.stem}>{q.stem}</div>
                        {q.options && (
                            <div className={styles.options}>
                                {q.options.map(opt => {
                                    const isSelected = selected.includes(opt.key);
                                    const isTargetAnswer = submitted && q.answer.includes(opt.key);
                                    
                                    let btnClass = styles.optionBtn;
                                    if (submitted) {
                                        if (isTargetAnswer) btnClass += ` ${styles.optionTarget}`;
                                        else if (isSelected) btnClass += ` ${styles.optionWrong}`;
                                    } else if (isSelected) {
                                        btnClass += ` ${styles.optionSelected}`;
                                    }

                                    return (
                                        <button 
                                            key={opt.key}
                                            className={btnClass}
                                            onClick={() => handleSelect(q.id, opt.key, q.type === 'mcq_multiple')}
                                            disabled={submitted}
                                        >
                                            <span className={styles.optionKey}>{opt.key}.</span> {opt.text}
                                        </button>
                                    );
                                })}
                            </div>
                        )}
                        {submitted && q.explanation && (
                            <div className={styles.explanation}>
                                <strong>解析: </strong>{q.explanation}
                            </div>
                        )}
                    </div>
                );
            })}
            {!submitted && (
                <button 
                    className={styles.submitBtn} 
                    onClick={handleSubmit}
                    disabled={Object.keys(answers).length !== quiz.questions.length}
                >
                    提交答案
                </button>
            )}
        </div>
    );
}
