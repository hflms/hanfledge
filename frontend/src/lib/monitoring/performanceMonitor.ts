/**
 * Production performance monitoring for skill renderers.
 * Tracks key metrics and reports to backend analytics.
 */

export interface PerformanceMetrics {
  componentName: string;
  renderTime: number;
  messageCount: number;
  memoryUsage?: number;
  timestamp: number;
}

export interface PerformanceThresholds {
  maxRenderTime: number;
  maxMessageCount: number;
  maxMemoryMB: number;
}

const DEFAULT_THRESHOLDS: PerformanceThresholds = {
  maxRenderTime: 100, // ms
  maxMessageCount: 1000,
  maxMemoryMB: 50,
};

class PerformanceMonitor {
  private metrics: Map<string, PerformanceMetrics[]> = new Map();
  private thresholds: PerformanceThresholds;

  constructor(thresholds: Partial<PerformanceThresholds> = {}) {
    this.thresholds = { ...DEFAULT_THRESHOLDS, ...thresholds };
  }

  track(componentName: string, renderTime: number, messageCount: number) {
    const metric: PerformanceMetrics = {
      componentName,
      renderTime,
      messageCount,
      memoryUsage: this.getMemoryUsage(),
      timestamp: Date.now(),
    };

    const existing = this.metrics.get(componentName) || [];
    existing.push(metric);
    
    // Keep last 100 metrics per component
    if (existing.length > 100) {
      existing.shift();
    }
    
    this.metrics.set(componentName, existing);

    // Check thresholds
    this.checkThresholds(metric);
  }

  private checkThresholds(metric: PerformanceMetrics) {
    if (metric.renderTime > this.thresholds.maxRenderTime) {
      console.warn(`[PerformanceMonitor] Slow render: ${metric.componentName} took ${metric.renderTime}ms`);
    }

    if (metric.messageCount > this.thresholds.maxMessageCount) {
      console.warn(`[PerformanceMonitor] High message count: ${metric.componentName} has ${metric.messageCount} messages`);
    }

    if (metric.memoryUsage && metric.memoryUsage > this.thresholds.maxMemoryMB) {
      console.warn(`[PerformanceMonitor] High memory: ${metric.componentName} using ${metric.memoryUsage}MB`);
    }
  }

  private getMemoryUsage(): number | undefined {
    if ('memory' in performance && performance.memory) {
      return Math.round((performance.memory as { usedJSHeapSize: number }).usedJSHeapSize / 1024 / 1024);
    }
    return undefined;
  }

  getMetrics(componentName?: string): PerformanceMetrics[] {
    if (componentName) {
      return this.metrics.get(componentName) || [];
    }
    return Array.from(this.metrics.values()).flat();
  }

  getAverageRenderTime(componentName: string): number {
    const metrics = this.metrics.get(componentName) || [];
    if (metrics.length === 0) return 0;
    
    const sum = metrics.reduce((acc, m) => acc + m.renderTime, 0);
    return Math.round(sum / metrics.length);
  }

  clear(componentName?: string) {
    if (componentName) {
      this.metrics.delete(componentName);
    } else {
      this.metrics.clear();
    }
  }
}

export const performanceMonitor = new PerformanceMonitor();
