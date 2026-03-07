import { useCallback } from 'react';
import { apiFetch } from './api';

export interface TelemetryEvent {
  event_type: 'custom' | 'skill_switch' | 'kp_transition' | 'session_start' | 'session_end' | 'milestone' | 'scaffold_change';
  from_state?: string;
  to_state?: string;
  trigger_reason?: string;
  kp_id?: number;
  skill_id: string;
  metadata?: Record<string, unknown>;
}

export function useTelemetry(sessionId: number, courseId?: number) {
  const trackEvent = useCallback(
    async (event: TelemetryEvent) => {
      try {
        await apiFetch('/student/telemetry', {
          method: 'POST',
          body: JSON.stringify({
            session_id: sessionId,
            course_id: courseId || 0,
            ...event,
          }),
        });
      } catch (error) {
        console.error('Failed to send telemetry event', error);
      }
    },
    [sessionId, courseId]
  );

  return { trackEvent };
}
