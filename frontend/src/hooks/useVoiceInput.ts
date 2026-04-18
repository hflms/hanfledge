import { useState, useRef, useCallback, useEffect } from 'react';
import type { AgentWebSocketChannel } from '@/lib/plugin/types';
import { createVAD, audioToBase64, type VADCallbacks } from '@/lib/vad';
import type { MicVAD } from '@ricky0123/vad-web';

interface UseVoiceInputProps {
  onTranscript: (text: string) => void;
  agentChannel: AgentWebSocketChannel;
  enableVAD?: boolean;
}

interface VoiceResultEvent {
  event: string;
  payload: {
    text: string;
    confidence: number;
    is_final: boolean;
  };
}

export function useVoiceInput({
  onTranscript,
  agentChannel,
  enableVAD = true,
}: UseVoiceInputProps) {
  const [recording, setRecording] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [vadActive, setVadActive] = useState(false); // VAD 检测到语音

  const mediaRecorderRef = useRef<MediaRecorder | null>(null);
  const streamRef = useRef<MediaStream | null>(null);
  const vadRef = useRef<MicVAD | null>(null);
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

  // -- Start Recording with VAD -----------------------------------------------

  const startRecording = useCallback(async () => {
    setError(null);

    try {
      if (enableVAD) {
        // VAD 模式: 使用 Silero VAD
        const vadCallbacks: VADCallbacks = {
          onSpeechStart: () => {
            if (!isMountedRef.current) return;
            console.log('[VAD] 检测到语音开始');
            setVadActive(true);
            sendWSEvent('voice_start', {
              sample_rate: 16000,
              format: 'pcm',
              language: 'zh-CN',
            });
          },
          onSpeechEnd: (audio: Float32Array) => {
            if (!isMountedRef.current) return;
            console.log('[VAD] 检测到语音结束, 样本数:', audio.length);
            setVadActive(false);

            // 发送音频数据
            const base64 = audioToBase64(audio);
            sendWSEvent('voice_data', { data: base64 });
            sendWSEvent('voice_end', {});
          },
          onVADMisfire: () => {
            console.log('[VAD] 误触发');
            setVadActive(false);
          },
          onError: (err: unknown) => {
            if (!isMountedRef.current) return;
            console.error('[VAD] 错误:', err);
            setError('VAD 初始化失败');
          },
        };

        const vad = await createVAD(vadCallbacks, {
          positiveSpeechThreshold: 0.8,
          minSpeechMs: 1000,
        });

        if (!isMountedRef.current) {
          vad.destroy();
          return;
        }

        vadRef.current = vad;
        vad.start();
        setRecording(true);

      } else {
        // 传统模式: 持续发送音频流
        const stream = await navigator.mediaDevices.getUserMedia({
          audio: {
            sampleRate: 16000,
            channelCount: 1,
            echoCancellation: true,
            noiseSuppression: true,
          },
        });

        if (!isMountedRef.current) {
          stream.getTracks().forEach((track) => track.stop());
          return;
        }

        streamRef.current = stream;

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

        mediaRecorder.start(250);
        setRecording(true);
      }
    } catch (err) {
      if (!isMountedRef.current) return;
      if (err instanceof DOMException && err.name === 'NotAllowedError') {
        setError('麦克风权限被拒绝，请在浏览器设置中允许访问');
      } else if (err instanceof DOMException && err.name === 'NotFoundError') {
        setError('未检测到麦克风设备');
      } else {
        setError('无法启动录音');
        console.error(err);
      }
    }
  }, [sendWSEvent, enableVAD]);

  // -- Stop Recording -----------------------------------------------

  const stopRecording = useCallback(() => {
    // 停止 VAD
    if (vadRef.current) {
      vadRef.current.pause();
      vadRef.current.destroy();
      vadRef.current = null;
    }

    // 停止传统录音
    const mediaRecorder = mediaRecorderRef.current;
    if (mediaRecorder && mediaRecorder.state !== 'inactive') {
      mediaRecorder.stop();
    }

    const stream = streamRef.current;
    if (stream) {
      stream.getTracks().forEach((track) => track.stop());
      streamRef.current = null;
    }

    if (!enableVAD) {
      sendWSEvent('voice_end', {});
    }

    mediaRecorderRef.current = null;
    setRecording(false);
    setVadActive(false);
  }, [sendWSEvent, enableVAD]);

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

  return {
    recording,
    vadActive,
    error,
    toggleRecording: handleToggle,
  };
}
