'use client';

import React from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import styles from './MarkdownRenderer.module.css';

// -- Types -------------------------------------------------------

interface MarkdownRendererProps {
    content: string;
    isStreaming?: boolean;
}

// -- Component ---------------------------------------------------

export default function MarkdownRenderer({ content, isStreaming = false }: MarkdownRendererProps) {
    return (
        <div className={styles.markdown}>
            <ReactMarkdown
                remarkPlugins={[remarkGfm]}
                components={{
                    // Code blocks and inline code
                    code({ className, children, ...props }) {
                        const isBlock = className?.startsWith('language-');
                        const language = className?.replace('language-', '') || '';

                        if (isBlock) {
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
                                        <code className={styles.code} {...props}>
                                            {children}
                                        </code>
                                    </pre>
                                </div>
                            );
                        }

                        return (
                            <code className={styles.inlineCode} {...props}>
                                {children}
                            </code>
                        );
                    },

                    // Block-level elements
                    p({ children }) {
                        return <p className={styles.paragraph}>{children}</p>;
                    },
                    h1({ children }) {
                        return <h1 className={styles.heading}>{children}</h1>;
                    },
                    h2({ children }) {
                        return <h2 className={styles.heading}>{children}</h2>;
                    },
                    h3({ children }) {
                        return <h3 className={styles.heading}>{children}</h3>;
                    },
                    ul({ children }) {
                        return <ul className={styles.list}>{children}</ul>;
                    },
                    ol({ children }) {
                        return <ol className={styles.list}>{children}</ol>;
                    },
                    li({ children }) {
                        return <li className={styles.listItem}>{children}</li>;
                    },
                    blockquote({ children }) {
                        return <blockquote className={styles.blockquote}>{children}</blockquote>;
                    },
                    table({ children }) {
                        return (
                            <div className={styles.tableWrapper}>
                                <table className={styles.table}>{children}</table>
                            </div>
                        );
                    },
                    th({ children }) {
                        return <th className={styles.th}>{children}</th>;
                    },
                    td({ children }) {
                        return <td className={styles.td}>{children}</td>;
                    },
                    hr() {
                        return <hr className={styles.hr} />;
                    },
                    a({ href, children }) {
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
                    strong({ children }) {
                        return <strong className={styles.strong}>{children}</strong>;
                    },
                }}
            >
                {content}
            </ReactMarkdown>
            {isStreaming && <span className={styles.cursor} />}
        </div>
    );
}
