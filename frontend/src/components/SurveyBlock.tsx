'use client';

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
}

export default function SurveyBlock({ survey }: SurveyBlockProps) {
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
                                    <li key={option.key} className={styles.optionItem}>
                                        <span className={styles.optionKey}>{option.key}.</span>
                                        <span>{option.text}</span>
                                    </li>
                                ))}
                            </ul>
                        )}

                        {question.type === 'multiple_choice' && question.options && (
                            <ul className={styles.optionList}>
                                {question.options.map(option => (
                                    <li key={option.key} className={styles.optionItem}>
                                        <span className={styles.optionKey}>{option.key}.</span>
                                        <span>{option.text}</span>
                                    </li>
                                ))}
                            </ul>
                        )}

                        {question.type === 'likert_scale' && (
                            <div className={styles.likertRow}>
                                {(question.scale_labels || ['完全不同意', '不太同意', '一般', '比较同意', '非常同意']).map((label, index) => (
                                    <span key={index} className={styles.likertOption}>
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
