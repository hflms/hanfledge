'use client';

import ErrorBoundaryFallback from '@/components/ErrorBoundaryFallback';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <ErrorBoundaryFallback
      title="出了点问题"
      fallbackMessage="页面发生了未知错误，请稍后重试。"
      error={error}
      reset={reset}
    />
  );
}
