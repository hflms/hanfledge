'use client';

import { useState, useCallback } from 'react';
import styles from './ChemistryEditor.module.css';

interface ChemistryEditorProps {
  initialValue?: string;
  onChange?: (equation: string) => void;
  readOnly?: boolean;
}

export function ChemistryEditor({ initialValue = '', onChange, readOnly = false }: ChemistryEditorProps) {
  const [equation, setEquation] = useState(initialValue);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const value = e.target.value;
      setEquation(value);
      onChange?.(value);
    },
    [onChange],
  );

  // Quick-insert chemical symbols
  const insertSymbol = useCallback(
    (symbol: string) => {
      setEquation((prev) => prev + symbol);
      onChange?.(equation + symbol);
    },
    [equation, onChange],
  );

  const symbols = ['→', '⇌', '↑', '↓', '·', 'Δ', '°C', 'mol', 'H₂O', 'CO₂', 'O₂', 'H₂'];

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>⚗</span>
        <span className={styles.title}>化学方程式编辑器</span>
      </div>
      <div className={styles.toolbar}>
        {symbols.map((sym) => (
          <button key={sym} className={styles.symbolBtn} onClick={() => insertSymbol(sym)} disabled={readOnly}>
            {sym}
          </button>
        ))}
      </div>
      <div className={styles.body}>
        <textarea
          className={styles.input}
          value={equation}
          onChange={handleChange}
          readOnly={readOnly}
          placeholder="输入化学方程式，例如: 2H₂ + O₂ → 2H₂O"
          rows={3}
        />
      </div>
      <div className={styles.footer}>
        <span className={styles.hint}>提示: 将来会集成分子结构可视化</span>
      </div>
    </div>
  );
}
