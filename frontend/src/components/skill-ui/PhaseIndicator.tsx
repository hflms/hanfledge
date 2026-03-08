import styles from './PhaseIndicator.module.css';

interface PhaseIndicatorProps<T extends string> {
  phases: readonly T[];
  currentPhase: T;
  labels: Record<T, string>;
}

export default function PhaseIndicator<T extends string>({
  phases,
  currentPhase,
  labels,
}: PhaseIndicatorProps<T>) {
  const currentIndex = phases.indexOf(currentPhase);

  return (
    <div className={styles.container}>
      {phases.map((phase, index) => {
        const isActive = index === currentIndex;
        const isCompleted = index < currentIndex;
        
        return (
          <div key={phase} className={styles.step}>
            <div 
              className={`${styles.circle} ${
                isActive ? styles.active : isCompleted ? styles.completed : ''
              }`}
            >
              {isCompleted ? '✓' : index + 1}
            </div>
            <div className={styles.label}>{labels[phase]}</div>
            {index < phases.length - 1 && (
              <div className={`${styles.line} ${isCompleted ? styles.completed : ''}`} />
            )}
          </div>
        );
      })}
    </div>
  );
}
