import React, { useEffect, useState, useRef } from 'react';
import ChatInputArea from '@/components/ChatInputArea';
import type { SkillRendererProps } from '@/lib/plugin/types';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import styles from './SteppedLearning.module.css';

export default function SteppedLearningRenderer({
    studentContext,
    knowledgePoint,
    agentChannel,
}: SkillRendererProps) {
    const [pages, setPages] = useState<{ id: string; content: string }[]>([]);
    const [currentPageIndex, setCurrentPageIndex] = useState(0);
    const [input, setInput] = useState('');
    const [waitingForAI, setWaitingForAI] = useState(false);
    const [streamingContent, setStreamingContent] = useState('');
    const latestPageRef = useRef('');

    useEffect(() => {
        const handleMessage = (data: string) => {
            try {
                const event = JSON.parse(data);
                switch (event.event) {
                    case 'token_delta': {
                        const payload = event.payload;
                        const delta = payload.text || payload.content || payload.delta || '';
                        setStreamingContent(prev => {
                            const next = prev + delta;
                            latestPageRef.current = next;
                            return next;
                        });
                        break;
                    }
                    case 'turn_complete': {
                        setWaitingForAI(false);
                        if (latestPageRef.current) {
                            setPages(prev => [
                                ...prev,
                                { id: `page-${Date.now()}`, content: latestPageRef.current },
                            ]);
                            setStreamingContent('');
                            latestPageRef.current = '';
                            // Automatically move to the new page when complete
                            setCurrentPageIndex(prev => prev + 1);
                        }
                        break;
                    }
                    case 'error': {
                        setWaitingForAI(false);
                        break;
                    }
                }
            } catch (err) {
                console.error('SteppedLearningRenderer Error:', err);
            }
        };

        const unsub = agentChannel.onMessage(handleMessage);
        return () => unsub();
    }, [agentChannel]);

    // Initial prompt kick off if no pages yet
    useEffect(() => {
        if (pages.length === 0 && !waitingForAI && streamingContent === '') {
            setWaitingForAI(true);
            agentChannel.send(
                JSON.stringify({
                    event: 'user_message',
                    payload: { text: "START_LESSON" },
                    timestamp: Math.floor(Date.now() / 1000),
                })
            );
        }
    }, [pages, agentChannel, waitingForAI, streamingContent]);

    const handleNext = () => {
        if (currentPageIndex < pages.length - 1) {
            setCurrentPageIndex(prev => prev + 1);
        }
    };

    const handlePrev = () => {
        if (currentPageIndex > 0) {
            setCurrentPageIndex(prev => prev - 1);
        }
    };

    const handleSend = (e: React.FormEvent) => {
        e.preventDefault();
        if (!input.trim() || waitingForAI) return;

        // We add user's answer/response to the pages as a small card, then wait for AI's next page
        setPages(prev => [
            ...prev,
            { id: `user-${Date.now()}`, content: `**你的回答**:\n\n${input}` }
        ]);
        setCurrentPageIndex(pages.length); // Point to the user response page we just added

        // Request AI's evaluation and next page
        setWaitingForAI(true);
        agentChannel.send(
            JSON.stringify({
                event: 'user_message',
                payload: { text: input },
                timestamp: Math.floor(Date.now() / 1000),
            })
        );
        setInput('');
    };

    // The active content to show
    const activeContent =
        (streamingContent && currentPageIndex >= pages.length)
            ? streamingContent
            : pages[currentPageIndex]?.content || '';

    const isStreamingOnPage = (streamingContent.length > 0 && currentPageIndex >= pages.length);

    return (
        <div className={styles.steppedContainer}>
            <div className={styles.pageHeader}>
                <div className={styles.progressText}>
                    {streamingContent ? `正在生成第 ${pages.length + 1} 页...` : `当前页: ${currentPageIndex + 1} / ${Math.max(1, pages.length)}`}
                </div>
            </div>

            <div className={styles.pageContentArea}>
                {activeContent ? (
                    <div className={`markdown-body ${styles.markdownWrapper}`}>
                        <ReactMarkdown remarkPlugins={[remarkGfm]}>
                            {activeContent}
                        </ReactMarkdown>
                        {isStreamingOnPage && <span className={styles.cursor} />}
                    </div>
                ) : (
                    <div className={styles.loadingArea}>
                        加载页面中...
                    </div>
                )}
            </div>

            <div className={styles.pageControls}>
                <div className={styles.navButtons}>
                    <button
                        className="btn btn-secondary"
                        onClick={handlePrev}
                        disabled={currentPageIndex === 0 || waitingForAI}
                    >
                        &larr; 上一页
                    </button>
                    <button
                        className="btn btn-secondary"
                        onClick={handleNext}
                        disabled={currentPageIndex >= pages.length - 1} // Can't go forward if on the latest or streaming
                    >
                        下一页 &rarr;
                    </button>
                </div>

                {/* Input */}
            <ChatInputArea
                input={input}
                setInput={setInput}
                sending={waitingForAI}
                onSend={() => handleSend({ preventDefault: () => {} } as React.FormEvent)}
                placeholder={waitingForAI ? '处理中...' : '输入消息...'}
            />
            </div>
        </div>
    );
}