'use client';

import ErrorBoundaryFallback from '@/components/ErrorBoundaryFallback';

export default function StudentError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <ErrorBoundaryFallback
      title="学生端出错了"
      fallbackMessage="页面加载异常，请刷新后重试。"
      error={error}
      reset={reset}
    />
  );
}
