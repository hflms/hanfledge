'use client';

import React, { useEffect } from 'react';
import styles from './SkillModal.module.css';

interface SkillModalProps {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    fullscreen?: boolean;
}

export default function SkillModal({ isOpen, onClose, title, children, fullscreen = false }: SkillModalProps) {
    const titleId = React.useId();

    useEffect(() => {
        if (!isOpen) return;
        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose();
            }
        };
        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, [isOpen, onClose]);

    if (!isOpen) return null;

    return (
        <div className={styles.overlay} onClick={onClose}>
            <div 
                className={`${styles.modal} ${fullscreen ? styles.modalFullscreen : ''}`}
                onClick={(e) => e.stopPropagation()}
                role="dialog"
                aria-modal="true"
                aria-labelledby={titleId}
            >
                <div className={styles.header}>
                    <h2 id={titleId} className={styles.title}>{title}</h2>
                    <button
                        className={styles.closeBtn}
                        onClick={onClose}
                        title="关闭 (ESC)"
                        aria-label="关闭对话框"
                    >
                        <span aria-hidden="true">✕</span>
                    </button>
                </div>
                <div className={styles.content}>
                    {children}
                </div>
            </div>
        </div>
    );
}
