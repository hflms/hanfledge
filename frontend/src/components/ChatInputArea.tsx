import React, { useState, useRef, useCallback } from 'react';
import styles from './ChatInputArea.module.css';

interface Props {
    input: string;
    setInput: (val: string) => void;
    sending: boolean;
    onSend: () => void;
    placeholder?: string;
    showToggle?: boolean;
}

export default function ChatInputArea({ input, setInput, sending, onSend, placeholder = '输入消息...', showToggle = true }: Props) {
    const [showKeyboard, setShowKeyboard] = useState(!showToggle);
    const inputRef = useRef<HTMLTextAreaElement>(null);

    const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            onSend();
            if (inputRef.current) {
                inputRef.current.style.height = 'auto';
            }
        }
    };

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
        setInput(e.target.value);
        const textarea = e.target;
        textarea.style.height = 'auto';
        textarea.style.height = Math.min(textarea.scrollHeight, 120) + 'px';
    }, [setInput]);

    const handleSendClick = () => {
        onSend();
        if (inputRef.current) {
            inputRef.current.style.height = 'auto';
        }
    };

    if (showToggle && !showKeyboard) {
        return (
            <div className={styles.toggleContainer}>
                <button 
                    onClick={() => setShowKeyboard(true)}
                    className={styles.toggleBtn}
                >
                    ⌨️ 切换手动输入...
                </button>
            </div>
        );
    }

    return (
        <div className={styles.inputArea}>
            <textarea
                ref={inputRef}
                className={styles.chatInput}
                value={input}
                onChange={handleInputChange}
                onKeyDown={handleKeyDown}
                placeholder={sending ? 'AI 正在处理...' : placeholder}
                disabled={sending}
                rows={1}
            />
            <div className={styles.actionRow}>
                <button
                    className={styles.sendBtn}
                    onClick={handleSendClick}
                    disabled={!input.trim() || sending}
                >
                    发送
                </button>
                {showToggle && (
                    <button
                        className={styles.collapseBtn}
                        onClick={() => setShowKeyboard(false)}
                        title="收起键盘"
                    >
                        收起
                    </button>
                )}
            </div>
        </div>
    );
}
