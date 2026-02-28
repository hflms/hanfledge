'use client';

import { useState, useCallback } from 'react';
import styles from './CodePlayground.module.css';

interface CodePlaygroundProps {
  initialCode?: string;
  language?: string;
  onChange?: (code: string) => void;
  readOnly?: boolean;
}

const LANGUAGES = [
  { id: 'python', name: 'Python' },
  { id: 'javascript', name: 'JavaScript' },
  { id: 'c', name: 'C' },
  { id: 'java', name: 'Java' },
];

export function CodePlayground({
  initialCode = '',
  language: defaultLang = 'python',
  onChange,
  readOnly = false,
}: CodePlaygroundProps) {
  const [code, setCode] = useState(initialCode);
  const [language, setLanguage] = useState(defaultLang);
  const [output, setOutput] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const value = e.target.value;
      setCode(value);
      onChange?.(value);
    },
    [onChange],
  );

  const handleRun = useCallback(async () => {
    setRunning(true);
    setOutput(null);

    // Placeholder: actual execution will be via sandboxed backend
    setTimeout(() => {
      setOutput(`[${language}] 代码执行功能即将上线\n\n输入代码:\n${code.substring(0, 200)}${code.length > 200 ? '...' : ''}`);
      setRunning(false);
    }, 500);
  }, [code, language]);

  return (
    <div className={styles.container}>
      <div className={styles.header}>
        <span className={styles.icon}>⌨</span>
        <span className={styles.title}>代码练习场</span>
        <select
          className={styles.langSelect}
          value={language}
          onChange={(e) => setLanguage(e.target.value)}
          disabled={readOnly}
        >
          {LANGUAGES.map((lang) => (
            <option key={lang.id} value={lang.id}>{lang.name}</option>
          ))}
        </select>
        <button className={styles.runBtn} onClick={handleRun} disabled={readOnly || running}>
          {running ? '运行中...' : '▶ 运行'}
        </button>
      </div>
      <div className={styles.body}>
        <div className={styles.editorPanel}>
          <textarea
            className={styles.codeInput}
            value={code}
            onChange={handleChange}
            readOnly={readOnly}
            placeholder={`# 在这里编写 ${LANGUAGES.find((l) => l.id === language)?.name || ''} 代码`}
            spellCheck={false}
          />
        </div>
        <div className={styles.outputPanel}>
          <label className={styles.label}>输出</label>
          <pre className={styles.output}>
            {output || '点击"运行"按钮执行代码'}
          </pre>
        </div>
      </div>
    </div>
  );
}
