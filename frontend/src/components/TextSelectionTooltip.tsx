'use client';
import React, { useEffect, useState } from 'react';
import styles from './TextSelectionTooltip.module.css';

interface Props {
  onAsk: (text: string) => void;
}

export default function TextSelectionTooltip({ onAsk }: Props) {
  const [position, setPosition] = useState<{ top: number; left: number } | null>(null);
  const [selectedText, setSelectedText] = useState('');

  useEffect(() => {
    const handleMouseUp = () => {
      // Slight delay to let the selection settle
      setTimeout(() => {
        const selection = window.getSelection();
        if (!selection || selection.isCollapsed) {
          setPosition(null);
          setSelectedText('');
          return;
        }

        const text = selection.toString().trim();
        if (text && text.length > 0) {
          // Check if selection is within our learning area to avoid triggering everywhere
          // Usually the whole app is inside main, but we can just check if it's not in an input
          const activeEl = document.activeElement;
          if (activeEl && (activeEl.tagName === 'INPUT' || activeEl.tagName === 'TEXTAREA')) {
              return; // Don't show tooltip when selecting text inside inputs
          }

          const range = selection.getRangeAt(0);
          const rect = range.getBoundingClientRect();
          
          setPosition({
            top: rect.top + window.scrollY - 40, // 40px above
            left: rect.left + window.scrollX + rect.width / 2, // Centered above selection
          });
          setSelectedText(text);
        } else {
          setPosition(null);
          setSelectedText('');
        }
      }, 10);
    };

    const handleMouseDown = () => {
      // If clicking outside the tooltip, hide it
      // The tooltip itself handles stopping propagation in its own mousedown
      setPosition(null);
    };

    document.addEventListener('mouseup', handleMouseUp);
    document.addEventListener('mousedown', handleMouseDown);

    return () => {
      document.removeEventListener('mouseup', handleMouseUp);
      document.removeEventListener('mousedown', handleMouseDown);
    };
  }, []);

  if (!position || !selectedText) return null;

  return (
    <div
      className={styles.tooltip}
      style={{ top: position.top, left: position.left }}
      onMouseDown={(e) => {
        // Prevent losing selection when clicking the button
        e.preventDefault();
        e.stopPropagation();
      }}
      onClick={(e) => {
        e.preventDefault();
        e.stopPropagation();
        onAsk(selectedText);
        setPosition(null);
        window.getSelection()?.removeAllRanges();
      }}
      title="对选中的内容提问"
    >
      ✨ 问 AI
    </div>
  );
}
