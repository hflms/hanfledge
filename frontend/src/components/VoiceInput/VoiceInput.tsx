'use client';

import type { AgentWebSocketChannel } from '@/lib/plugin/types';
import styles from './VoiceInput.module.css';
import { useVoiceInput } from '../../hooks/useVoiceInput';

// -- Types -----------------------------------------------

interface VoiceInputProps {
  onTranscript: (text: string) => void;
  agentChannel: AgentWebSocketChannel;
  disabled?: boolean;
  /** 启用 VAD (Voice Activity Detection) */
  enableVAD?: boolean;
}

// -- VoiceInput Component -----------------------------------------------

export default function VoiceInput({ 
  onTranscript, 
  agentChannel, 
  disabled = false,
  enableVAD = true, // 默认启用 VAD
}: VoiceInputProps) {
  const { recording, vadActive, error, toggleRecording } = useVoiceInput({
    agentChannel,
    onTranscript,
    enableVAD,
  });

  return (
    <div className={styles.container}>
      <button
        className={`${styles.micButton} ${recording ? styles.recording : ''} ${vadActive ? styles.vadActive : ''}`}
        onClick={toggleRecording}
        disabled={disabled}
        aria-label={recording ? '停止录音' : '开始录音'}
        title={
          recording 
            ? (enableVAD ? '停止录音 (VAD 模式)' : '停止录音') 
            : '开始录音'
        }
        type="button"
      >
        {recording && <span className={styles.pulse} aria-hidden="true" />}
        {vadActive && <span className={styles.vadPulse} aria-hidden="true" />}
        <svg
          className={styles.micIcon}
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          aria-hidden="true"
        >
          <path d="M12 1a3 3 0 0 0-3 3v8a3 3 0 0 0 6 0V4a3 3 0 0 0-3-3z" />
          <path d="M19 10v2a7 7 0 0 1-14 0v-2" />
          <line x1="12" y1="19" x2="12" y2="23" />
          <line x1="8" y1="23" x2="16" y2="23" />
        </svg>
      </button>
      {recording && (
        <span className={styles.recordingText}>
          {enableVAD 
            ? (vadActive ? '🎤 检测到语音...' : '🎧 等待语音...') 
            : '录音中...'}
        </span>
      )}
      {error && <span className={styles.errorText}>{error}</span>}
    </div>
  );
}
