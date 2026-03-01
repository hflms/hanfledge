'use client';

import { useEffect, useRef, useState, useCallback } from 'react';
import type { AgentWebSocketChannel } from '@/lib/plugin/types';
import styles from './Avatar3D.module.css';

// -- Types -----------------------------------------------

interface Avatar3DProps {
  agentChannel: AgentWebSocketChannel;
  active?: boolean;
}

type AvatarState = 'idle' | 'speaking' | 'gesturing' | 'thinking';

interface AvatarActionEvent {
  event: string;
  payload: {
    action: string;
    params: Record<string, unknown>;
    duration_ms: number;
  };
}

// -- Avatar3D Component -----------------------------------------------

export default function Avatar3D({ agentChannel, active = true }: Avatar3DProps) {
  const [avatarState, setAvatarState] = useState<AvatarState>('idle');
  const [statusText, setStatusText] = useState('待命中');
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // -- Handle Avatar Actions -----------------------------------------------

  const handleAction = useCallback((action: string, params: Record<string, unknown>, durationMs: number) => {
    // Clear any previous action timeout
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }

    switch (action) {
      case 'speak':
        setAvatarState('speaking');
        setStatusText((params.text as string) || '讲解中...');
        break;
      case 'gesture':
        setAvatarState('gesturing');
        setStatusText('演示中...');
        break;
      case 'expression':
        setStatusText((params.expression as string) || '');
        break;
      case 'point':
        setAvatarState('gesturing');
        setStatusText('指向要点...');
        break;
      default:
        setAvatarState('idle');
        setStatusText('待命中');
    }

    // Return to idle after duration
    if (durationMs > 0) {
      timeoutRef.current = setTimeout(() => {
        setAvatarState('idle');
        setStatusText('待命中');
      }, durationMs);
    }
  }, []);

  // -- WebSocket Listener -----------------------------------------------

  useEffect(() => {
    if (!active) return;

    const handleMessage = (dataString: string) => {
      try {
        const data: AvatarActionEvent = JSON.parse(dataString);
        if (data.event === 'avatar_action' && data.payload) {
          handleAction(
            data.payload.action,
            data.payload.params || {},
            data.payload.duration_ms || 0,
          );
        }
      } catch {
        // Ignore non-JSON messages
      }
    };

    const unsubscribe = agentChannel.onMessage(handleMessage);
    return () => {
      unsubscribe();
    };
  }, [agentChannel, handleAction, active]);

  // Cleanup timeout on unmount
  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  if (!active) {
    return null;
  }

  return (
    <div className={styles.container}>
      <div className={styles.avatarStage}>
        {/* Placeholder 2D avatar using CSS — to be replaced with WebGL/Three.js */}
        <div className={`${styles.avatar} ${styles[avatarState]}`}>
          {/* Head */}
          <div className={styles.head}>
            {/* Eyes */}
            <div className={styles.eyes}>
              <div className={styles.eye} />
              <div className={styles.eye} />
            </div>
            {/* Mouth */}
            <div
              className={`${styles.mouth} ${avatarState === 'speaking' ? styles.mouthSpeaking : ''}`}
            />
          </div>
          {/* Body */}
          <div className={styles.body} />
        </div>
      </div>

      {/* Status indicator */}
      <div className={styles.statusBar}>
        <span
          className={`${styles.statusDot} ${avatarState !== 'idle' ? styles.statusActive : ''}`}
          aria-hidden="true"
        />
        <span className={styles.statusText}>{statusText}</span>
      </div>
    </div>
  );
}
