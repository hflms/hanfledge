/**
 * Voice Activity Detection (VAD) using Silero VAD (WebAssembly)
 * 
 * 只在检测到人声时才发送音频数据,大幅降低后端 ASR 的无效计算开销。
 */

import { MicVAD } from "@ricky0123/vad-web"

export interface VADConfig {
  /** 语音检测阈值 (0-1),越高越严格 */
  positiveSpeechThreshold?: number
  /** 静音检测阈值 (0-1) */
  negativeSpeechThreshold?: number
  /** 最小语音持续时间 (毫秒) */
  minSpeechMs?: number
  /** 前置缓冲时间 (毫秒) */
  preSpeechPadMs?: number
  /** 后置缓冲时间 (毫秒) */
  redemptionMs?: number
}

export interface VADCallbacks {
  onSpeechStart?: () => void
  onSpeechEnd?: (audio: Float32Array) => void
  onVADMisfire?: () => void
  onError?: (error: Error) => void
}

/**
 * 创建 VAD 实例
 */
export async function createVAD(
  callbacks: VADCallbacks,
  config: VADConfig = {}
): Promise<MicVAD> {
  const {
    positiveSpeechThreshold = 0.8,
    negativeSpeechThreshold = 0.5,
    minSpeechMs = 1000,
    preSpeechPadMs = 300,
    redemptionMs = 1000,
  } = config

  try {
    const vad = await MicVAD.new({
      positiveSpeechThreshold,
      negativeSpeechThreshold,
      minSpeechMs,
      preSpeechPadMs,
      redemptionMs,
      onSpeechStart: callbacks.onSpeechStart,
      onSpeechEnd: callbacks.onSpeechEnd,
      onVADMisfire: callbacks.onVADMisfire,
    })

    return vad
  } catch (error) {
    callbacks.onError?.(error as Error)
    throw error
  }
}

/**
 * 将 Float32Array 音频转换为 Base64 (用于 WebSocket 传输)
 */
export function audioToBase64(audio: Float32Array): string {
  // 转换为 Int16 PCM
  const int16 = new Int16Array(audio.length)
  for (let i = 0; i < audio.length; i++) {
    const s = Math.max(-1, Math.min(1, audio[i]))
    int16[i] = s < 0 ? s * 0x8000 : s * 0x7fff
  }

  // 转换为 Base64
  const bytes = new Uint8Array(int16.buffer)
  let binary = ""
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
}
