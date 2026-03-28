import { useState, useCallback, useRef, useEffect } from 'react';

/**
 * Shared message management hook for all skill renderers.
 * Handles message state, auto-scroll, and message limits.
 */

export interface ChatMessage {
  id: string;
  role: 'student' | 'coach' | 'system' | 'teacher';
  content: string;
  timestamp: number;
}

interface UseMessagesOptions {
  maxMessages?: number;
  autoScroll?: boolean;
  initialMessages?: ChatMessage[];
}

export function useMessages(options: UseMessagesOptions = {}) {
  const { maxMessages = 100, autoScroll = true, initialMessages = [] } = options;
  const [messages, setMessages] = useState<ChatMessage[]>(initialMessages);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const addMessage = useCallback((msg: ChatMessage) => {
    setMessages(prev => {
      const updated = [...prev, msg];
      if (updated.length > maxMessages) {
        return updated.slice(-maxMessages);
      }
      return updated;
    });
  }, [maxMessages]);

  const clearMessages = useCallback(() => {
    setMessages([]);
  }, []);

  const scrollToBottom = useCallback(() => {
    if (autoScroll) {
      messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [autoScroll]);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  return {
    messages,
    addMessage,
    clearMessages,
    messagesEndRef,
    scrollToBottom,
  };
}
