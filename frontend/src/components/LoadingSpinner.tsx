import styles from './LoadingSpinner.module.css';

type SpinnerSize = 'small' | 'medium' | 'large' | 'fullscreen';

interface LoadingSpinnerProps {
    /** Controls the wrapper padding. Default: 'medium' (60px) */
    size?: SpinnerSize;
    /** Accessibility label. Default: '加载中...' */
    'aria-label'?: string;
}

/**
 * Shared loading spinner with consistent sizing.
 *
 * Variants:
 * - small:      40px vertical padding
 * - medium:     60px vertical padding (default)
 * - large:      80px vertical padding
 * - fullscreen: 100vh height, vertically centered
 */
export default function LoadingSpinner({ size = 'medium', 'aria-label': ariaLabel = '加载中...' }: LoadingSpinnerProps) {
    return (
        <div className={`${styles.wrapper} ${styles[size]}`} role="status" aria-label={ariaLabel}>
            <div className={styles.spinner} aria-hidden="true" />
        </div>
    );
}
