import styles from '../../../loading.module.css';

export default function SessionLoading() {
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <div className={styles.spinner} />
        <p className={styles.text}>正在连接学习会话…</p>
      </div>
    </div>
  );
}
