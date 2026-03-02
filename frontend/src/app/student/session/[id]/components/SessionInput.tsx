import { useRef, useCallback } from 'react';
import dynamic from 'next/dynamic';
import type { AgentWebSocketChannel } from '@/lib/plugin/types';
import styles from '../page.module.css';

const VoiceInput = dynamic(() => import('@/components/VoiceInput/VoiceInput'), { ssr: false });


interface SessionInputProps {
    input: string;
    setInput: (value: string) => void;
    sending: boolean;
    sessionActive: boolean;
    onSend: () => void;
    agentChannel: AgentWebSocketChannel;
    providerOverride?: string;
    setProviderOverride?: (value: string) => void;
    modelOverride?: string;
    setModelOverride?: (value: string) => void;
}



export default function SessionInput({
    input,
    setInput,
    sending,
    sessionActive,
    onSend,
    agentChannel,
    providerOverride,
    setProviderOverride,
    modelOverride,
    setModelOverride,
}: SessionInputProps) {

    const inputRef = useRef<HTMLTextAreaElement>(null);

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
        setInput(e.target.value);
        const textarea = e.target;
        textarea.style.height = 'auto';
        textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
    }, [setInput]);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            onSend();
        }
    };

    const handleVoiceTranscript = useCallback((text: string) => {
        if (!text.trim()) return;
        setInput(text);
        inputRef.current?.focus();
    }, [setInput]);

    
    return (
        <div className={styles.inputAreaWrapper}>
            {setProviderOverride && (
                <div className={styles.overrideControls}>
                    <select 
                        value={providerOverride || ''} 
                        onChange={e => {
                            setProviderOverride(e.target.value);
                            if (setModelOverride) setModelOverride(''); // reset model when provider changes
                        }}
                        className={styles.overrideSelect}
                    >
                        <option value="">默认全局 AI</option>
                        <option value="ollama">本地 Ollama</option>
                        <option value="dashscope">通义千问 (DashScope)</option>
                    </select>
                    {providerOverride === 'ollama' && setModelOverride && (
                        <input 
                            type="text" 
                            value={modelOverride || ''} 
                            onChange={e => setModelOverride(e.target.value)}
                            placeholder="模型 (如 qwen2.5:7b)"
                            className={styles.overrideInput}
                        />
                    )}
                    {providerOverride === 'dashscope' && setModelOverride && (
                        <input 
                            type="text" 
                            value={modelOverride || ''} 
                            onChange={e => setModelOverride(e.target.value)}
                            placeholder="模型 (如 qwen-max)"
                            className={styles.overrideInput}
                        />
                    )}
                </div>
            )}
            <div className={styles.inputArea}>

            <VoiceInput
                onTranscript={handleVoiceTranscript}
                agentChannel={agentChannel}
                disabled={!sessionActive || sending}
            />
            <textarea
                ref={inputRef}
                className={styles.chatInput}
                value={input}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder={
                    !sessionActive
                        ? '会话已结束'
                        : sending
                        ? 'AI 正在思考...'
                        : '输入你的想法或问题... (Enter 发送, Shift+Enter 换行)'
                }
                disabled={!sessionActive || sending}
                rows={1}
            />
            <button
                className={`btn btn-primary ${styles.sendBtn}`}
                onClick={onSend}
                disabled={!input.trim() || sending || !sessionActive}
            >
                发送
            </button>
        </div>
        </div>
    );
}
