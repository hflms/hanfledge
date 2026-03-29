'use client';

import type { InstructionalDesigner } from '@/lib/api';
import styles from '../page.module.css';

// -- Props --------------------------------------------------------

interface AutonomousTabProps {
  designers: InstructionalDesigner[];
  designerId: string;
  deadline: string;
  allowRetry: boolean;
  maxAttempts: number;
  disabled: boolean;
  onDesignerChange: (value: string) => void;
  onDeadlineChange: (value: string) => void;
  onAllowRetryChange: (value: boolean) => void;
  onMaxAttemptsChange: (value: number) => void;
}

// -- Component ----------------------------------------------------

export default function AutonomousTab({
  designers,
  designerId,
  deadline,
  allowRetry,
  maxAttempts,
  disabled,
  onDesignerChange,
  onDeadlineChange,
  onAllowRetryChange,
  onMaxAttemptsChange,
}: AutonomousTabProps) {
  return (
    <div className={styles.autonomousPanel}>
      <div className={styles.autonomousInfo}>
        全自主学习模式下，学生将在 AI 教学设计者的引导下自主完成学习目标。
        系统会根据已挂载的技能和学生的掌握程度，动态调整脚手架层级和学习路径。
      </div>

      {/* Designer selection */}
      <div className={styles.formGroup}>
        <label className={styles.formLabel}>教学设计者</label>
        <p className={styles.formHint}>选择引导学生学习的 AI 设计者风格</p>
        <select
          className={styles.designerSelect}
          value={designerId}
          onChange={(e) => onDesignerChange(e.target.value)}
          disabled={disabled}
          name="designer-id"
        >
          <option value="">-- 选择设计者 --</option>
          {designers.map((d) => (
            <option key={d.id} value={d.id}>
              {d.name} — {d.description}
            </option>
          ))}
        </select>
      </div>

      {/* Settings row */}
      <div className={styles.settingsRow}>
        <div className={styles.formGroup}>
          <label className={styles.formLabel}>截止日期</label>
          <input
            type="datetime-local"
            className={styles.formInput}
            value={deadline}
            onChange={(e) => onDeadlineChange(e.target.value)}
            disabled={disabled}
            name="deadline"
          />
        </div>

        <div className={styles.formGroup}>
          <label className={styles.formLabel}>最大尝试次数</label>
          <input
            type="number"
            className={styles.formInput}
            value={maxAttempts}
            min={1}
            max={10}
            onChange={(e) => onMaxAttemptsChange(Number(e.target.value))}
            disabled={disabled}
            name="max-attempts"
          />
        </div>
      </div>

      {/* Allow retry toggle */}
      <div className={styles.formGroup}>
        <label className={styles.checkboxLabel}>
          <input
            type="checkbox"
            checked={allowRetry}
            onChange={(e) => onAllowRetryChange(e.target.checked)}
            disabled={disabled}
          />
          允许学生重试
        </label>
      </div>
    </div>
  );
}
