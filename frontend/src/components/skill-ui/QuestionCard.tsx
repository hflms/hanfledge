import { ReactNode } from 'react';
import styles from './QuestionCard.module.css';

interface QuestionCardProps {
  number: number;
  stem: string;
  children: ReactNode;
  status?: 'unanswered' | 'answered' | 'correct' | 'incorrect';
}

export default function QuestionCard({
  number,
  stem,
  children,
  status = 'unanswered',
}: QuestionCardProps) {
  return (
    <div className={`${styles.card} ${styles[status]}`}>
      <div className={styles.header}>
        <span className={styles.number}>题目 {number}</span>
        {status === 'correct' && <span className={styles.badge}>✓ 正确</span>}
        {status === 'incorrect' && <span className={styles.badge}>✗ 错误</span>}
      </div>
      <div className={styles.stem}>{stem}</div>
      <div className={styles.content}>{children}</div>
    </div>
  );
}
