'use client';

import styles from './error.module.css';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <div className={styles.container}>
      <div className={styles.card}>
        <div className={styles.icon}>⚠</div>
        <h2 className={styles.title}>出了点问题</h2>
        <p className={styles.message}>
          {error.message || '页面发生了未知错误，请稍后重试。'}
        </p>
        {error.digest && (
          <p className={styles.digest}>错误代码: {error.digest}</p>
        )}
        <button className={styles.button} onClick={reset}>
          重新加载
        </button>
      </div>
    </div>
  );
}
