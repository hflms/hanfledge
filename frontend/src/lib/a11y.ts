// -- Accessibility Utilities ----------------------------------
//
// Shared helpers for keyboard navigation, focus trapping, and
// ARIA-compliant interactive elements.

import { useEffect, useRef, type KeyboardEvent as ReactKeyboardEvent } from 'react';

// -- Modal Focus Trap & ESC Close -----------------------------

/**
 * useModalA11y - manages ESC-to-close and focus trapping for modal dialogs.
 *
 * Usage:
 *   const modalRef = useModalA11y(isOpen, onClose);
 *   <div ref={modalRef} role="dialog" aria-modal="true" aria-labelledby="...">
 *
 * Features:
 *   - ESC key closes the modal
 *   - Tab/Shift+Tab cycles within the modal (focus trap)
 *   - Auto-focuses first focusable element on open
 *   - Restores focus to trigger element on close
 */
export function useModalA11y(isOpen: boolean, onClose: () => void) {
    const modalRef = useRef<HTMLDivElement>(null);
    const previousFocusRef = useRef<HTMLElement | null>(null);

    useEffect(() => {
        if (!isOpen) return;

        // Save currently focused element to restore later
        previousFocusRef.current = document.activeElement as HTMLElement;

        // Auto-focus first focusable element inside modal
        const timer = setTimeout(() => {
            const modal = modalRef.current;
            if (!modal) return;
            const focusable = getFocusableElements(modal);
            if (focusable.length > 0) {
                focusable[0].focus();
            } else {
                modal.focus();
            }
        }, 0);

        return () => clearTimeout(timer);
    }, [isOpen]);

    // Restore focus on close
    useEffect(() => {
        if (!isOpen && previousFocusRef.current) {
            previousFocusRef.current.focus();
            previousFocusRef.current = null;
        }
    }, [isOpen]);

    // ESC close + focus trap
    useEffect(() => {
        if (!isOpen) return;

        const handleKeyDown = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                e.preventDefault();
                onClose();
                return;
            }

            if (e.key === 'Tab') {
                const modal = modalRef.current;
                if (!modal) return;

                const focusable = getFocusableElements(modal);
                if (focusable.length === 0) {
                    e.preventDefault();
                    return;
                }

                const first = focusable[0];
                const last = focusable[focusable.length - 1];

                if (e.shiftKey) {
                    if (document.activeElement === first) {
                        e.preventDefault();
                        last.focus();
                    }
                } else {
                    if (document.activeElement === last) {
                        e.preventDefault();
                        first.focus();
                    }
                }
            }
        };

        document.addEventListener('keydown', handleKeyDown);
        return () => document.removeEventListener('keydown', handleKeyDown);
    }, [isOpen, onClose]);

    return modalRef;
}

/** Get all focusable elements within a container. */
function getFocusableElements(container: HTMLElement): HTMLElement[] {
    const selector = [
        'a[href]',
        'button:not([disabled])',
        'input:not([disabled])',
        'select:not([disabled])',
        'textarea:not([disabled])',
        '[tabindex]:not([tabindex="-1"])',
    ].join(', ');
    return Array.from(container.querySelectorAll<HTMLElement>(selector));
}

// -- Interactive Card Keyboard Support ------------------------

/**
 * handleCardKeyDown - enables keyboard activation for div-based buttons/cards.
 *
 * Usage:
 *   <div role="button" tabIndex={0} onClick={onClick} onKeyDown={handleCardKeyDown}>
 *
 * Triggers the element's click handler on Enter or Space.
 */
export function handleCardKeyDown(e: ReactKeyboardEvent<HTMLElement>) {
    if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        e.currentTarget.click();
    }
}

/**
 * cardA11yProps - returns standard a11y props for interactive cards/divs.
 *
 * Usage:
 *   <div {...cardA11yProps} onClick={handler}>...</div>
 */
export const cardA11yProps = {
    role: 'button' as const,
    tabIndex: 0,
    onKeyDown: handleCardKeyDown,
};
