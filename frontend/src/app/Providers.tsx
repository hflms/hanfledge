'use client';

import type { ReactNode } from 'react';
import { ToastProvider } from '@/components/Toast';
import { ThemeProvider } from '@/lib/plugin/themes';

export function Providers({ children }: { children: ReactNode }) {
  return (
    <ThemeProvider>
      <ToastProvider>
        {children}
      </ToastProvider>
    </ThemeProvider>
  );
}

export default Providers;
