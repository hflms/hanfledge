import React, { useState, useId } from 'react';
import styles from './SkillTestOverlay.module.css';

interface SkillTestOverlayProps {
    question: string;
    onClose: () => void;
    onSubmit: (answer: string) => void;
}

export default function SkillTestOverlay({ question, onClose, onSubmit }: SkillTestOverlayProps) {
    const [answer, setAnswer] = useState('');
    const titleId = useId();

    const handleSubmit = () => {
        if (!answer.trim()) return;
        onSubmit(answer);
    };

    return (
        <div className={styles.overlay}>
            <div
                className={styles.card}
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
            >
                <div className={styles.header} id={titleId}>🎓 阶段性能力测试</div>
                <div className={styles.body}>
                    <p className={styles.questionText}>{question}</p>
                    <textarea 
                        className={styles.textarea}
                        placeholder="请输入你的解答..."
                        value={answer}
                        onChange={(e) => setAnswer(e.target.value)}
                        rows={5}
                    />
                </div>
                <div className={styles.footer}>
                    <button type="button" className={styles.cancelBtn} onClick={onClose}>先不测了</button>
                    <button type="button" className={styles.submitBtn} onClick={handleSubmit} disabled={!answer.trim()}>
                        提交答案
                    </button>
                </div>
            </div>
        </div>
    );
}
