import { useState, useEffect, useCallback, useRef } from 'react';
import type { AgentWebSocketChannel } from '../types';

/**
 * Unified WebSocket agent channel hook.
 * Handles message streaming, thinking status, and cleanup.
 */

interface UseAgentChannelOptions {
  onMessage?: (content: string) => void;
  onThinking?: (status: string) => void;
  onComplete?: () => void;
  onScaffoldChange?: (data: unknown) => void;
}

export function useAgentChannel(
  agentChannel: AgentWebSocketChannel,
  options: UseAgentChannelOptions = {}
) {
  const [sending, setSending] = useState(false);
  const [thinkingStatus, setThinkingStatus] = useState<string | null>(null);
  const [streamingContent, setStreamingContent] = useState('');
  const unsubscribeRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    const unsubscribe = agentChannel.onMessage((data: string) => {
      try {
        const event = JSON.parse(data);
        
        switch (event.event) {
          case 'agent_thinking': {
            const status = event.payload?.status || '思考中...';
            setThinkingStatus(status);
            options.onThinking?.(status);
            break;
          }
          
          case 'token_delta': {
            const text = event.payload?.text || '';
            setThinkingStatus(null);
            setStreamingContent(prev => prev + text);
            break;
          }
          
          case 'turn_complete': {
            const content = streamingContent || event.payload?.content || '';
            setStreamingContent('');
            setSending(false);
            options.onMessage?.(content);
            options.onComplete?.();
            break;
          }
          
          case 'ui_scaffold_change': {
            options.onScaffoldChange?.(event.payload);
            break;
          }
        }
      } catch (err) {
        console.error('[useAgentChannel] Parse error:', err);
      }
    });

    unsubscribeRef.current = unsubscribe;
    return () => unsubscribe();
  }, [agentChannel, streamingContent, options]);

  const send = useCallback(async (message: string) => {
    if (sending) return;
    
    setSending(true);
    setStreamingContent('');
    
    try {
      await agentChannel.send(message);
    } catch (err) {
      console.error('[useAgentChannel] Send error:', err);
      setSending(false);
    }
  }, [agentChannel, sending]);

  return {
    send,
    sending,
    thinkingStatus,
    streamingContent,
  };
}
