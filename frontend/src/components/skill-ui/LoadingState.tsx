import { ReactNode } from 'react';
import styles from './LoadingState.module.css';

interface LoadingStateProps {
  message?: string;
  progress?: number;
  onCancel?: () => void;
  children?: ReactNode;
}

export default function LoadingState({
  message = '加载中...',
  progress,
  onCancel,
  children,
}: LoadingStateProps) {
  return (
    <div className={styles.container} role="status" aria-label={message}>
      <div className={styles.spinner} aria-hidden="true" />
      <p className={styles.message}>{message}</p>
      
      {progress !== undefined && (
        <div
          className={styles.progressBar}
          role="progressbar"
          aria-valuenow={progress}
          aria-valuemin={0}
          aria-valuemax={100}
        >
          <div 
            className={styles.progressFill} 
            style={{ width: `${progress}%` }}
          />
        </div>
      )}
      
      {children}
      
      {onCancel && (
        <button className={styles.cancelBtn} onClick={onCancel}>
          取消
        </button>
      )}
    </div>
  );
}
