'use client';

import { useState, useRef, useCallback, useEffect } from 'react';
import type { AgentWebSocketChannel } from '@/lib/plugin/types';
import styles from './VoiceInput.module.css';

// -- Types -----------------------------------------------

interface VoiceInputProps {
  onTranscript: (text: string) => void;
  agentChannel: AgentWebSocketChannel;
  disabled?: boolean;
}

interface VoiceResultEvent {
  event: string;
  payload: {
    text: string;
    confidence: number;
    is_final: boolean;
  };
}

// -- VoiceInput Component -----------------------------------------------

export default function VoiceInput({ onTranscript, agentChannel, disabled = false }: VoiceInputProps) {
  const [recording, setRecording] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const isMountedRef = useRef(true);

  // Track mount status to prevent async state updates on unmounted component
  useEffect(() => {
    isMountedRef.current = true;
    return () => {
      isMountedRef.current = false;
    };
  }, []);

  // Listen for voice_result events from WebSocket via agentChannel
  useEffect(() => {
    const handleMessage = (data: string) => {
      try {
        const parsed: VoiceResultEvent = JSON.parse(data);
        if (parsed.event === 'voice_result' && parsed.payload) {
          if (parsed.payload.is_final && parsed.payload.text) {
            onTranscript(parsed.payload.text);
          }
        }
      } catch {
        // Ignore non-JSON messages
      }
    };

    const unsubscribe = agentChannel.onMessage(handleMessage);
    return () => {
      unsubscribe();
    };
  }, [agentChannel, onTranscript]);

  // -- Send WS Event Helper -----------------------------------------------

  const sendWSEvent = useCallback(
    (event: string, payload: Record<string, unknown>) => {
      agentChannel.send(JSON.stringify({ event, payload, timestamp: Date.now() }));
    },
    [agentChannel],
  );

  // -- Start Recording -----------------------------------------------

  const startRecording = useCallback(async () => {
    setError(null);

    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: {
          sampleRate: 16000,
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
        },
      });

      // If unmounted while waiting for permissions, stop stream immediately
      if (!isMountedRef.current) {
        stream.getTracks().forEach((track) => track.stop());
        return;
      }

      streamRef.current = stream;

      // Notify server that voice input is starting
      sendWSEvent('voice_start', {
        sample_rate: 16000,
        format: 'webm',
        language: 'zh-CN',
      });

      const mediaRecorder = new MediaRecorder(stream, {
        mimeType: 'audio/webm;codecs=opus',
      });
      mediaRecorderRef.current = mediaRecorder;

      mediaRecorder.ondataavailable = (e: BlobEvent) => {
        if (!isMountedRef.current) return;
        if (e.data.size > 0) {
          // Convert blob to base64 and send through WebSocket
          const reader = new FileReader();
          reader.onloadend = () => {
            if (!isMountedRef.current) return;
            const base64 = (reader.result as string).split(',')[1];
            if (base64) {
              sendWSEvent('voice_data', { data: base64 });
            }
          };
          reader.readAsDataURL(e.data);
        }
      };

      mediaRecorder.start(250); // Send chunks every 250ms
      setRecording(true);
    } catch (err) {
      if (!isMountedRef.current) return;
      if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('麦克风权限被拒绝，请在浏览器设置中允许访问');
      } else if (err instanceof DOMException && err.name === 'NotFoundError') {
        setError('未检测到麦克风设备');
      } else {
        setError('无法启动录音');
      }
    }
  }, [sendWSEvent]);

  // -- Stop Recording -----------------------------------------------

  const stopRecording = useCallback(() => {
    const mediaRecorder = mediaRecorderRef.current;
    if (mediaRecorder && mediaRecorder.state !== 'inactive') {
      mediaRecorder.stop();
    }

    const stream = streamRef.current;
    if (stream) {
      stream.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }

    // Notify server that voice input has ended
    sendWSEvent('voice_end', {});

    mediaRecorderRef.current = null;
    setRecording(false);
  }, [sendWSEvent]);

  // -- Toggle Handler -----------------------------------------------

  const isStartingRef = useRef(false);

  const handleToggle = useCallback(async () => {
    if (isStartingRef.current) return;

    if (recording) {
      stopRecording();
    } else {
      isStartingRef.current = true;
      try {
        await startRecording();
      } finally {
        isStartingRef.current = false;
      }
    }
  }, [recording, startRecording, stopRecording]);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (mediaRecorderRef.current && mediaRecorderRef.current.state !== 'inactive') {
        mediaRecorderRef.current.stop();
      }
      if (streamRef.current) {
        streamRef.current.getTracks().forEach((track) => track.stop());
      }
    };
  }, []);

  return (
    <div className={styles.container}>
      <button
        className={`${styles.micButton} ${recording ? styles.recording : ''}`}
        onClick={handleToggle}
        disabled={disabled}
        aria-label={recording ? '停止录音' : '开始录音'}
        title={recording ? '停止录音' : '开始录音'}
        type="button"
      >
        {recording && <span className={styles.pulse} />}
        <svg
          className={styles.micIcon}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z" />
          <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
          <line x1="12" y1="19" x2="12" y2="23" />
          <line x1="8" y1="23" x2="16" y2="23" />
        </svg>
      </button>
      {recording && <span className={styles.recordingText}>录音中...</span>}
      {error && <span className={styles.errorText}>{error}</span>}
    </div>
  );
}
