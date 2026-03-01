import styles from './ErrorBoundaryFallback.module.css';

interface ErrorBoundaryFallbackProps {
    /** Title shown above the error message */
    title: string;
    /** Fallback message when `error.message` is empty */
    fallbackMessage: string;
    /** The error object from the error boundary */
    error: Error & { digest?: string };
    /** Reset function from the error boundary */
    reset: () => void;
}

/**
 * Shared error boundary fallback UI.
 *
 * Used by `app/error.tsx`, `app/teacher/error.tsx`, and `app/student/error.tsx`
 * with different titles and fallback messages.
 */
export default function ErrorBoundaryFallback({
    title,
    fallbackMessage,
    error,
    reset,
}: ErrorBoundaryFallbackProps) {
    return (
        <div className={styles.container}>
            <div className={styles.card}>
                <div className={styles.icon}>⚠</div>
                <h2 className={styles.title}>{title}</h2>
                <p className={styles.message}>
                    {error.message || fallbackMessage}
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
