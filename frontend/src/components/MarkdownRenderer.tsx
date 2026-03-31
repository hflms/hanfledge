'use client';

import React, { useMemo } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import rehypeRaw from 'rehype-raw';
import MermaidDiagram from './MermaidDiagram';
import styles from './MarkdownRenderer.module.css';

// -- Types -------------------------------------------------------

interface MarkdownRendererProps {
    content: string;
    isStreaming?: boolean;
}

// -- Hoisted Constants -------------------------------------------

const remarkPluginsList = [remarkGfm, remarkMath];
const rehypePluginsList = [rehypeRaw, rehypeKatex];

// Custom tags emitted by the AI that should never reach the DOM.
// If one slips through stripping, render its children as-is.
const CUSTOM_TAGS = ['slides', 'suggestions', 'presentation', 'reasoning', 'thinking', 'analysis', 'skill_output'] as const;
type CustomTag = typeof CUSTOM_TAGS[number];

function PassthroughTag({ children }: { children?: React.ReactNode }) {
    return <>{children}</>;
}

function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = React.useState(false);
    const timeoutRef = React.useRef<NodeJS.Timeout>(undefined);

    const handleCopy = React.useCallback(() => {
        navigator.clipboard.writeText(text);
        setCopied(true);

        if (timeoutRef.current) {
            clearTimeout(timeoutRef.current);
        }

        timeoutRef.current = setTimeout(() => {
            setCopied(false);
        }, 2000);
    }, [text]);

    // Cleanup timeout on unmount
    React.useEffect(() => {
        return () => {
            if (timeoutRef.current) {
                clearTimeout(timeoutRef.current);
            }
        };
    }, []);

    return (
        <button
            className={styles.copyBtn}
            onClick={handleCopy}
            type="button"
            aria-label={copied ? "已复制代码" : "复制代码"}
            aria-live="polite"
        >
            {copied ? '已复制!' : '复制'}
        </button>
    );
}

// -- Component ---------------------------------------------------

const MarkdownRenderer = React.memo(function MarkdownRenderer({ content, isStreaming = false }: MarkdownRendererProps) {
    const markdownComponents = useMemo(() => ({
        // Suppress unknown custom tags from AI output
        ...Object.fromEntries(CUSTOM_TAGS.map(tag => [tag, PassthroughTag])) as Record<CustomTag, typeof PassthroughTag>,

        // Code blocks and inline code
        code({ className, children }: { className?: string; children?: React.ReactNode }) {
            const isBlock = className?.startsWith('language-');
            const language = className?.replace('language-', '') || '';

            if (isBlock) {
                if (language === 'mermaid') {
                    return <MermaidDiagram chart={String(children).replace(/\n$/, '')} />;
                }

                return (
                    <div className={styles.codeBlock}>
                        {language && (
                            <div className={styles.codeBlockHeader}>
                                <span className={styles.codeLanguage}>{language}</span>
                                <CopyButton text={String(children).replace(/\n$/, '')} />
                            </div>
                        )}
                        <pre className={styles.pre}>
                            <code className={styles.code}>
                                {children}
                            </code>
                        </pre>
                    </div>
                );
            }

            return (
                <code className={styles.inlineCode}>
                    {children}
                </code>
            );
        },

        // Block-level elements
        p({ children }: { children?: React.ReactNode }) {
            return <p className={styles.paragraph}>{children}</p>;
        },
        h1({ children }: { children?: React.ReactNode }) {
            return <h1 className={styles.heading}>{children}</h1>;
        },
        h2({ children }: { children?: React.ReactNode }) {
            return <h2 className={styles.heading}>{children}</h2>;
        },
        h3({ children }: { children?: React.ReactNode }) {
            return <h3 className={styles.heading}>{children}</h3>;
        },
        ul({ children }: { children?: React.ReactNode }) {
            return <ul className={styles.list}>{children}</ul>;
        },
        ol({ children }: { children?: React.ReactNode }) {
            return <ol className={styles.list}>{children}</ol>;
        },
        li({ children }: { children?: React.ReactNode }) {
            return <li className={styles.listItem}>{children}</li>;
        },
        blockquote({ children }: { children?: React.ReactNode }) {
            return <blockquote className={styles.blockquote}>{children}</blockquote>;
        },
        table({ children }: { children?: React.ReactNode }) {
            return (
                <div className={styles.tableWrapper}>
                    <table className={styles.table}>{children}</table>
                </div>
            );
        },
        th({ children }: { children?: React.ReactNode }) {
            return <th className={styles.th}>{children}</th>;
        },
        td({ children }: { children?: React.ReactNode }) {
            return <td className={styles.td}>{children}</td>;
        },
        hr() {
            return <hr className={styles.hr} />;
        },
        a({ href, children }: { href?: string; children?: React.ReactNode }) {
            return (
                <a
                    className={styles.link}
                    href={href}
                    target="_blank"
                    rel="noopener noreferrer"
                >
                    {children}
                </a>
            );
        },
        strong({ children }: { children?: React.ReactNode }) {
            return <strong className={styles.strong}>{children}</strong>;
        },
    }), []);

    return (
        <div className={styles.markdown}>
            <ReactMarkdown
                remarkPlugins={remarkPluginsList}
                rehypePlugins={rehypePluginsList}
                components={markdownComponents}
            >
                {content}
            </ReactMarkdown>
            {isStreaming && <span className={styles.cursor} />}
        </div>
    );
});

export default MarkdownRenderer;
