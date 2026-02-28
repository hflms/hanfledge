import styles from '../../loading.module.css';

export default function DashboardLoading() {
  return (
    <div className={styles.skeletons}>
      <div className={styles.skeletonRow}>
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
      </div>
      <div className={styles.skeletonWide} />
      <div className={styles.skeletonRow}>
        <div className={styles.skeletonCard} />
        <div className={styles.skeletonCard} />
      </div>
    </div>
  );
}
