import { useMemo } from 'react';
import type { ChatMessage } from './useMessages';

/**
 * Optimized message filtering and grouping.
 * Memoizes expensive operations to prevent re-computation.
 */

export function useMessageOptimization(messages: ChatMessage[]) {
  // Group messages by role for batch rendering
  const messagesByRole = useMemo(() => {
    const groups: Record<string, ChatMessage[]> = {
      student: [],
      coach: [],
      system: [],
      teacher: [],
    };
    
    messages.forEach(msg => {
      if (groups[msg.role]) {
        groups[msg.role].push(msg);
      }
    });
    
    return groups;
  }, [messages]);

  // Calculate statistics
  const stats = useMemo(() => ({
    total: messages.length,
    byRole: {
      student: messagesByRole.student.length,
      coach: messagesByRole.coach.length,
      system: messagesByRole.system.length,
      teacher: messagesByRole.teacher?.length || 0,
    },
  }), [messages.length, messagesByRole]);

  // Filter messages by time range
  const filterByTimeRange = useMemo(() => {
    return (startTime: number, endTime: number) => {
      return messages.filter(
        msg => msg.timestamp >= startTime && msg.timestamp <= endTime
      );
    };
  }, [messages]);

  // Search messages
  const searchMessages = useMemo(() => {
    return (query: string) => {
      const lowerQuery = query.toLowerCase();
      return messages.filter(msg =>
        msg.content.toLowerCase().includes(lowerQuery)
      );
    };
  }, [messages]);

  return {
    messagesByRole,
    stats,
    filterByTimeRange,
    searchMessages,
  };
}
