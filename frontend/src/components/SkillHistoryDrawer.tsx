'use client';

import React, { useState } from 'react';
import styles from './SkillHistoryDrawer.module.css';

export interface SkillHistoryItem {
    id: string;
    type: 'presentation' | 'quiz' | 'survey' | 'other';
    title: string;
    timestamp: number;
    icon: string;
}

interface SkillHistoryDrawerProps {
    items: SkillHistoryItem[];
    onItemClick: (item: SkillHistoryItem) => void;
}

export default function SkillHistoryDrawer({ items, onItemClick }: SkillHistoryDrawerProps) {
    const [isOpen, setIsOpen] = useState(false);
    const drawerId = React.useId();

    return (
        <>
            {/* Toggle Button */}
            <button 
                className={styles.toggleBtn}
                onClick={() => setIsOpen(!isOpen)}
                title={isOpen ? '收起历史' : '展开历史'}
                aria-label={isOpen ? '收起历史' : '展开历史'}
                aria-expanded={isOpen}
                aria-controls={drawerId}
            >
                <span aria-hidden="true">{isOpen ? '→' : '←'}</span>
            </button>

            {/* Drawer */}
            <div id={drawerId} className={`${styles.drawer} ${isOpen ? styles.drawerOpen : ''}`}>
                <div className={styles.drawerHeader}>
                    <h3 className={styles.drawerTitle}>📚 生成历史</h3>
                    <span className={styles.itemCount}>{items.length}</span>
                </div>
                <div className={styles.drawerContent}>
                    {items.length === 0 ? (
                        <div className={styles.emptyState}>
                            <span className={styles.emptyIcon}>📭</span>
                            <p className={styles.emptyText}>暂无生成内容</p>
                        </div>
                    ) : (
                        <div className={styles.itemList}>
                            {items.map((item) => (
                                <button
                                    key={item.id}
                                    className={styles.historyItem}
                                    onClick={() => onItemClick(item)}
                                >
                                    <span className={styles.itemIcon}>{item.icon}</span>
                                    <div className={styles.itemInfo}>
                                        <div className={styles.itemTitle}>{item.title}</div>
                                        <div className={styles.itemTime}>
                                            {new Date(item.timestamp).toLocaleTimeString('zh-CN', {
                                                hour: '2-digit',
                                                minute: '2-digit',
                                            })}
                                        </div>
                                    </div>
                                </button>
                            ))}
                        </div>
                    )}
                </div>
            </div>
        </>
    );
}
