import Link from 'next/link';
import styles from './layout.module.css';

export default function HelpLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className={styles.container}>
      <aside className={styles.sidebar}>
        <h2 className={styles.title}>操作手册</h2>
        <nav className={styles.nav}>
          <Link href="/help/student" className={styles.link}>学生指南</Link>
          <Link href="/help/teacher" className={styles.link}>教师指南</Link>
          <Link href="/help/school_admin" className={styles.link}>学校管理员指南</Link>
          <Link href="/help/sys_admin" className={styles.link}>系统管理员指南</Link>
          <div className={styles.divider}></div>
          <Link href="/" className={styles.backLink}>← 返回系统</Link>
        </nav>
      </aside>
      <main className={styles.content}>
        {children}
      </main>
    </div>
  );
}
