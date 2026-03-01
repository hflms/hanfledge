'use client';

import { useState } from 'react';
import { handleCardKeyDown } from '@/lib/a11y';
import styles from './ErrorDiagnosisRenderer.module.css';

interface DiagnosisNode {
  type: 'error' | 'root_cause' | 'knowledge_gap' | 'remedy';
  title: string;
  description: string;
  children?: DiagnosisNode[];
}

interface ErrorDiagnosisProps {
  diagnosis?: DiagnosisNode;
  knowledgeGaps?: string[];
  remedyPath?: string[];
  isActive?: boolean;
}

export function ErrorDiagnosisRenderer({
  diagnosis,
  knowledgeGaps = [],
  remedyPath = [],
  isActive = false,
}: ErrorDiagnosisProps) {
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());

  const toggleNode = (key: string) => {
    setExpandedNodes((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  if (!isActive) return null;

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>🔍</span>
        <span className={styles.title}>错误诊断</span>
      </div>

      {diagnosis && (
        <div className={styles.section}>
          <h4 className={styles.sectionTitle}>诊断树</h4>
          <div className={styles.tree}>
            <div
              className={`${styles.node} ${styles[diagnosis.type]}`}
              role="button"
              tabIndex={0}
              aria-expanded={expandedNodes.has('root')}
              onClick={() => toggleNode('root')}
              onKeyDown={handleCardKeyDown}
            >
              <span className={styles.nodeLabel}>{diagnosis.title}</span>
              <p className={styles.nodeDesc}>{diagnosis.description}</p>
            </div>
          </div>
        </div>
      )}

      {knowledgeGaps.length > 0 && (
        <div className={styles.section}>
          <h4 className={styles.sectionTitle}>知识缺口</h4>
          <ul className={styles.gapList}>
            {knowledgeGaps.map((gap, i) => (
              <li key={i} className={styles.gapItem}>
                <span className={styles.gapIcon}>⚠</span>
                {gap}
              </li>
            ))}
          </ul>
        </div>
      )}

      {remedyPath.length > 0 && (
        <div className={styles.section}>
          <h4 className={styles.sectionTitle}>补救路径</h4>
          <ol className={styles.remedyList}>
            {remedyPath.map((step, i) => (
              <li key={i} className={styles.remedyItem}>
                <span className={styles.stepNumber}>{i + 1}</span>
                {step}
              </li>
            ))}
          </ol>
        </div>
      )}

      {!diagnosis && knowledgeGaps.length === 0 && remedyPath.length === 0 && (
        <div className={styles.empty}>
          提交答案后，系统将自动分析错误原因
        </div>
      )}
    </div>
  );
}

export default ErrorDiagnosisRenderer;
