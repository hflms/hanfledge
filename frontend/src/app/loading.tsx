import styles from './loading.module.css';

export default function RootLoading() {
  return (
    <div className={styles.container}>
      <div className={styles.content}>
        <div className={styles.spinner} />
        <p className={styles.text}>加载中…</p>
      </div>
    </div>
  );
}
