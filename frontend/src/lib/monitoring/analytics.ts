/**
 * Production monitoring configuration and utilities.
 */

export interface MonitoringConfig {
  enabled: boolean;
  sampleRate: number; // 0-1, percentage of events to track
  reportInterval: number; // ms
  endpoint?: string; // Backend analytics endpoint
}

export const MONITORING_CONFIG: MonitoringConfig = {
  enabled: process.env.NODE_ENV === 'production',
  sampleRate: 0.1, // Track 10% of events
  reportInterval: 60000, // Report every 60s
  endpoint: '/api/v1/analytics/performance',
};

export function shouldSample(): boolean {
  return Math.random() < MONITORING_CONFIG.sampleRate;
}

export interface AnalyticsEvent {
  type: 'render' | 'interaction' | 'error' | 'performance';
  component: string;
  data: Record<string, unknown>;
  timestamp: number;
}

class AnalyticsQueue {
  private queue: AnalyticsEvent[] = [];
  private timer: NodeJS.Timeout | null = null;

  start() {
    if (!MONITORING_CONFIG.enabled || this.timer) return;

    this.timer = setInterval(() => {
      this.flush();
    }, MONITORING_CONFIG.reportInterval);
  }

  push(event: AnalyticsEvent) {
    if (!MONITORING_CONFIG.enabled || !shouldSample()) return;
    
    this.queue.push(event);
    
    // Flush if queue is large
    if (this.queue.length >= 50) {
      this.flush();
    }
  }

  async flush() {
    if (this.queue.length === 0) return;

    const events = [...this.queue];
    this.queue = [];

    try {
      await fetch(MONITORING_CONFIG.endpoint!, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ events }),
      });
    } catch (err) {
      console.error('[Analytics] Failed to send events:', err);
    }
  }

  stop() {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    this.flush();
  }
}

export const analyticsQueue = new AnalyticsQueue();

// Auto-start in production
if (typeof window !== 'undefined' && MONITORING_CONFIG.enabled) {
  analyticsQueue.start();
  
  // Flush on page unload
  window.addEventListener('beforeunload', () => {
    analyticsQueue.flush();
  });
}
