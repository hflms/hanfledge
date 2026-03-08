'use client';

import React from 'react';
import styles from './SkillModal.module.css';

interface SkillModalProps {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    fullscreen?: boolean;
}

export default function SkillModal({ isOpen, onClose, title, children, fullscreen = false }: SkillModalProps) {
    if (!isOpen) return null;

    return (
        <div className={styles.overlay} onClick={onClose}>
            <div 
                className={`${styles.modal} ${fullscreen ? styles.modalFullscreen : ''}`}
                onClick={(e) => e.stopPropagation()}
            >
                <div className={styles.header}>
                    <h2 className={styles.title}>{title}</h2>
                    <button className={styles.closeBtn} onClick={onClose} title="关闭 (ESC)">
                        ✕
                    </button>
                </div>
                <div className={styles.content}>
                    {children}
                </div>
            </div>
        </div>
    );
}
