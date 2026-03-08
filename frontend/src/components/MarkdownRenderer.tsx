'use client';

import React, { useMemo } from 'react';
import ReactMarkdown, { type Components } from 'react-markdown';
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

// -- Component ---------------------------------------------------

const MarkdownRenderer = React.memo(function MarkdownRenderer({ content, isStreaming = false }: MarkdownRendererProps) {
    const markdownComponents = useMemo(() => ({
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
                                <button
                                    className={styles.copyBtn}
                                    onClick={() => {
                                        const text = String(children).replace(/\n$/, '');
                                        navigator.clipboard.writeText(text);
                                    }}
                                    type="button"
                                >
                                    复制
                                </button>
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
