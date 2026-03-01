import { useEffect, useRef } from 'react';
import dynamic from 'next/dynamic';
import styles from '../page.module.css';

const MarkdownRenderer = dynamic(() => import('@/components/MarkdownRenderer'));

// -- Types -------------------------------------------------------

export interface ChatMessage {
    id: string;
    role: 'student' | 'coach' | 'system' | 'teacher';
    content: string;
    timestamp: number;
}

interface MessageListProps {
    messages: ChatMessage[];
    streamingContent: string;
    thinkingStatus: string | null;
}

// -- Component ---------------------------------------------------

export default function MessageList({ messages, streamingContent, thinkingStatus }: MessageListProps) {
    const messagesEndRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }, [messages, streamingContent, thinkingStatus]);

    return (
        <div className={styles.messagesArea}>
            {messages.length === 0 && !thinkingStatus && (
                <div className={styles.messageSystem}>
                    发送消息开始学习对话
                </div>
            )}

            {messages.map(msg => (
                <div
                    key={msg.id}
                    className={`${styles.messageBubble} ${
                        msg.role === 'student' ? styles.messageStudent :
                        msg.role === 'teacher' ? styles.messageTeacher :
                        msg.role === 'coach' ? styles.messageCoach :
                        styles.messageSystem
                    }`}
                >
                    {msg.role !== 'system' && (
                        <div className={styles.messageHeader}>
                            <span className={`${styles.roleIcon} ${
                                msg.role === 'student' ? styles.roleStudent : msg.role === 'teacher' ? styles.roleTeacher : styles.roleCoach
                            }`}>
                                {msg.role === 'student' ? 'S' : msg.role === 'teacher' ? 'T' : 'AI'}
                            </span>
                            <span className={styles.roleLabel}>
                                {msg.role === 'student' ? '我' : msg.role === 'teacher' ? '人类教师 (接管)' : 'AI 导师'}
                            </span>
                        </div>
                    )}
                    <div className={styles.messageContent}>
                        {msg.role === 'coach' ? (
                            <MarkdownRenderer content={msg.content} />
                        ) : (
                            msg.content
                        )}
                    </div>
                </div>
            ))}

            {/* Streaming content (partial coach response) */}
            {streamingContent && (
                <div className={`${styles.messageBubble} ${styles.messageCoach}`}>
                    <div className={styles.messageHeader}>
                        <span className={`${styles.roleIcon} ${styles.roleCoach}`}>AI</span>
                        <span className={styles.roleLabel}>AI 导师</span>
                    </div>
                    <div className={styles.messageContent}>
                        <MarkdownRenderer content={streamingContent} isStreaming />
                    </div>
                </div>
            )}

            {/* Agent thinking indicator */}
            {thinkingStatus && (
                <div className={styles.thinkingIndicator}>
                    <div className={styles.thinkingDots}>
                        <div className={styles.thinkingDot} />
                        <div className={styles.thinkingDot} />
                        <div className={styles.thinkingDot} />
                    </div>
                    <span>{thinkingStatus}</span>
                </div>
            )}

            <div ref={messagesEndRef} />
        </div>
    );
}
