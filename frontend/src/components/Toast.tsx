'use client';

import { createContext, useContext, useCallback, useState, useRef } from 'react';
import styles from './Toast.module.css';

// -- Types -------------------------------------------------------

type ToastVariant = 'success' | 'error' | 'warning' | 'info';

interface Toast {
  id: number;
  message: string;
  variant: ToastVariant;
}

interface ToastContextValue {
  toast: (message: string, variant?: ToastVariant) => void;
}

// -- Context -----------------------------------------------------

const ToastContext = createContext<ToastContextValue | null>(null);

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error('useToast must be used within <ToastProvider>');
  }
  return ctx;
}

// -- Icons -------------------------------------------------------

const ICONS: Record<ToastVariant, string> = {
  success: '\u2713',  // checkmark
  error: '\u2717',    // cross
  warning: '\u26A0',  // warning sign
  info: '\u2139',     // info sign
};

// -- Provider + Renderer -----------------------------------------

const TOAST_DURATION = 4000;

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const idRef = useRef(0);

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, []);

  const toast = useCallback(
    (message: string, variant: ToastVariant = 'info') => {
      const id = ++idRef.current;
      setToasts((prev) => [...prev, { id, message, variant }]);

      // Auto-dismiss
      setTimeout(() => removeToast(id), TOAST_DURATION);
    },
    [removeToast],
  );

  return (
    <ToastContext.Provider value={{ toast }}>
      {children}
      <div className={styles.overlay}>
        {toasts.map((t) => (
          <div key={t.id} className={`${styles.toast} ${styles[t.variant]}`}>
            <span className={styles.icon}>{ICONS[t.variant]}</span>
            <div className={styles.body}>
              <p className={styles.message}>{t.message}</p>
            </div>
            <button className={styles.close} onClick={() => removeToast(t.id)}>
              &times;
            </button>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}
