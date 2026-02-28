'use client';

import { useState, useCallback } from 'react';
import styles from './LaTeXEditor.module.css';

interface LaTeXEditorProps {
  initialValue?: string;
  onChange?: (latex: string) => void;
  readOnly?: boolean;
}

export function LaTeXEditor({ initialValue = '', onChange, readOnly = false }: LaTeXEditorProps) {
  const [latex, setLatex] = useState(initialValue);
  const [error, setError] = useState<string | null>(null);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const value = e.target.value;
      setLatex(value);
      setError(null);
      onChange?.(value);
    },
    [onChange],
  );

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>∑</span>
        <span className={styles.title}>LaTeX 公式编辑器</span>
      </div>
      <div className={styles.body}>
        <div className={styles.inputPanel}>
          <label className={styles.label}>LaTeX 输入</label>
          <textarea
            className={styles.input}
            value={latex}
            onChange={handleChange}
            readOnly={readOnly}
            placeholder="输入 LaTeX 公式，例如: \frac{-b \pm \sqrt{b^2 - 4ac}}{2a}"
            rows={4}
          />
        </div>
        <div className={styles.previewPanel}>
          <label className={styles.label}>预览</label>
          <div className={styles.preview}>
            {latex ? (
              <code className={styles.previewCode}>{latex}</code>
            ) : (
              <span className={styles.placeholder}>公式预览将显示在这里</span>
            )}
          </div>
          {error && <span className={styles.error}>{error}</span>}
        </div>
      </div>
      <div className={styles.footer}>
        <span className={styles.hint}>提示: 将来会集成 KaTeX 进行实时渲染</span>
      </div>
    </div>
  );
}
