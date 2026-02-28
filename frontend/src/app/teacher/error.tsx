'use client';

import styles from '../error.module.css';

export default function TeacherError({
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
        <h2 className={styles.title}>教师端出错了</h2>
        <p className={styles.message}>
          {error.message || '页面加载异常，请刷新后重试。'}
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
