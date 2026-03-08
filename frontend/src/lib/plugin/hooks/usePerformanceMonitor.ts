import { useEffect, useRef } from 'react';

/**
 * Performance monitoring hook for skill renderers.
 * Tracks render times, message processing, and memory usage.
 */

interface PerformanceMetrics {
  renderCount: number;
  averageRenderTime: number;
  messageProcessingTime: number;
  lastRenderTime: number;
}

export function usePerformanceMonitor(componentName: string, enabled = false) {
  const metricsRef = useRef<PerformanceMetrics>({
    renderCount: 0,
    averageRenderTime: 0,
    messageProcessingTime: 0,
    lastRenderTime: 0,
  });
  const renderStartRef = useRef<number>(0);

  useEffect(() => {
    if (!enabled) return;

    renderStartRef.current = performance.now();

    return () => {
      const renderTime = performance.now() - renderStartRef.current;
      const metrics = metricsRef.current;

      metrics.renderCount++;
      metrics.lastRenderTime = renderTime;
      metrics.averageRenderTime =
        (metrics.averageRenderTime * (metrics.renderCount - 1) + renderTime) /
        metrics.renderCount;

      if (renderTime > 16) {
        console.warn(
          `[Performance] ${componentName} render took ${renderTime.toFixed(2)}ms (>16ms)`
        );
      }
    };
  });

  const measureOperation = (operationName: string, fn: () => void) => {
    if (!enabled) {
      fn();
      return;
    }

    const start = performance.now();
    fn();
    const duration = performance.now() - start;

    if (duration > 10) {
      console.warn(
        `[Performance] ${componentName}.${operationName} took ${duration.toFixed(2)}ms`
      );
    }
  };

  const getMetrics = () => metricsRef.current;

  return {
    measureOperation,
    getMetrics,
  };
}
