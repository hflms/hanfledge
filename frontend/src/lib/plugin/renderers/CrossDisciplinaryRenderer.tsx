'use client';

import styles from './CrossDisciplinaryRenderer.module.css';

interface ConceptLink {
  sourceDiscipline: string;
  targetDiscipline: string;
  sourceConcept: string;
  targetConcept: string;
  connectionType: string;
  explanation: string;
}

interface CrossDisciplinaryProps {
  links?: ConceptLink[];
  transferQuestions?: string[];
  isActive?: boolean;
}

export function CrossDisciplinaryRenderer({
  links = [],
  transferQuestions = [],
  isActive = false,
}: CrossDisciplinaryProps) {
  if (!isActive) return null;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>🔗</span>
        <span className={styles.title}>跨学科关联</span>
      </div>

      {links.length > 0 && (
        <div className={styles.section}>
          <h4 className={styles.sectionTitle}>概念连接</h4>
          {links.map((link, i) => (
            <div key={i} className={styles.linkCard}>
              <div className={styles.disciplines}>
                <span className={styles.discipline}>{link.sourceDiscipline}</span>
                <span className={styles.arrow}>⟷</span>
                <span className={styles.discipline}>{link.targetDiscipline}</span>
              </div>
              <div className={styles.concepts}>
                <span className={styles.concept}>{link.sourceConcept}</span>
                <span className={styles.connType}>{link.connectionType}</span>
                <span className={styles.concept}>{link.targetConcept}</span>
              </div>
              <p className={styles.explanation}>{link.explanation}</p>
            </div>
          ))}
        </div>
      )}

      {transferQuestions.length > 0 && (
        <div className={styles.section}>
          <h4 className={styles.sectionTitle}>迁移思考题</h4>
          <ul className={styles.questionList}>
            {transferQuestions.map((q, i) => (
              <li key={i} className={styles.questionItem}>
                <span className={styles.qIcon}>💡</span>
                {q}
              </li>
            ))}
          </ul>
        </div>
      )}

      {links.length === 0 && transferQuestions.length === 0 && (
        <div className={styles.empty}>
          学习过程中，系统将自动发现跨学科关联
        </div>
      )}
    </div>
  );
}

export default CrossDisciplinaryRenderer;
