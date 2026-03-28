import { useState, useEffect, useLayoutEffect, useCallback, useRef } from 'react';
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

  // Use refs to avoid re-subscribing on every state/callback change.
  // This fixes the stale-closure bug where streamingContent was always ''
  // at the time turn_complete fired because the effect kept re-running.
  const streamingContentRef = useRef('');
  const optionsRef = useRef(options);

  useLayoutEffect(() => {
    optionsRef.current = options;
  });

  useEffect(() => {
    const unsubscribe = agentChannel.onMessage((data: string) => {
      try {
        const event = JSON.parse(data);
        
        switch (event.event) {
          case 'agent_thinking': {
            const status = event.payload?.status || '思考中...';
            setThinkingStatus(status);
            optionsRef.current.onThinking?.(status);
            break;
          }
          
          case 'token_delta': {
            const text = event.payload?.text || event.payload?.content || event.payload?.delta || '';
            if (!text) break;
            setThinkingStatus(null);
            setStreamingContent(prev => {
              const next = prev + text;
              streamingContentRef.current = next;
              return next;
            });
            break;
          }
          
          case 'turn_complete': {
            const content = streamingContentRef.current || event.payload?.content || '';
            streamingContentRef.current = '';
            setStreamingContent('');
            setSending(false);
            optionsRef.current.onMessage?.(content);
            optionsRef.current.onComplete?.();
            break;
          }
          
          case 'ui_scaffold_change': {
            optionsRef.current.onScaffoldChange?.(event.payload);
            break;
          }
        }
      } catch (err) {
        console.error('[useAgentChannel] Parse error:', err);
      }
    });

    return () => unsubscribe();
  }, [agentChannel]);

  const send = useCallback((text: string) => {
    if (sending) return;
    
    setSending(true);
    setStreamingContent('');
    streamingContentRef.current = '';

    // Wrap raw text into the WSEvent JSON format the backend expects.
    const wsEvent = JSON.stringify({
      event: 'user_message',
      payload: { text },
      timestamp: Math.floor(Date.now() / 1000),
    });
    
    try {
      agentChannel.send(wsEvent);
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
