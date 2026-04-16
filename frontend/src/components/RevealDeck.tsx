'use client';

import React, { useEffect, useRef, useState } from 'react';
import type { Api } from 'reveal.js';
import 'reveal.js/dist/reveal.css';
import 'reveal.js/dist/theme/white.css';
import styles from './RevealDeck.module.css';

interface RevealDeckProps {
    markdown: string;
    onSlideChange?: (indexh: number, indexv: number) => void;
    fullscreen?: boolean;
}

interface SlideChangeEvent {
    indexh: number;
    indexv: number;
}

export default function RevealDeck({ markdown, onSlideChange, fullscreen = false }: RevealDeckProps) {
    const deckRef = useRef<HTMLDivElement>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const revealInstance = useRef<Api | null>(null);
    const [isLoaded, setIsLoaded] = useState(false);
    const onSlideChangeRef = useRef(onSlideChange);

    useEffect(() => {
        onSlideChangeRef.current = onSlideChange;
    }, [onSlideChange]);

    // Handle fullscreen mode - just trigger layout recalculation
    useEffect(() => {
        if (revealInstance.current && fullscreen) {
            // Small delay to ensure CSS transition completes
            setTimeout(() => {
                revealInstance.current?.layout();
            }, 100);
        }
    }, [fullscreen]);

    useEffect(() => {
        let mounted = true;
        const initReveal = async () => {
            if (!deckRef.current) return;

            // Import Reveal.js dynamically
            const Reveal = (await import('reveal.js')).default;
            const RevealMarkdown = (await import('reveal.js/plugin/markdown/markdown.js')).default;
            const RevealNotes = (await import('reveal.js/plugin/notes/notes.js')).default;
            
            if (!mounted) return;

            revealInstance.current = new Reveal(deckRef.current, {
                plugins: [RevealMarkdown, RevealNotes],
                controls: true,
                progress: true,
                center: true,
                hash: false,
                embedded: !fullscreen,
                width: 960,
                height: 700,
                margin: 0.04,
                transition: 'slide',
                backgroundTransition: 'fade',
                slideNumber: 'c/t',
                keyboard: true,
                overview: true,
                touch: true,
                loop: false,
                rtl: false,
                navigationMode: 'default',
                shuffle: false,
                fragments: true,
                fragmentInURL: false,
                help: true,
                showNotes: false,
                autoPlayMedia: null,
                preloadIframes: null,
                autoAnimate: true,
                autoAnimateMatcher: null,
                autoAnimateEasing: 'ease',
                autoAnimateDuration: 1.0,
                autoAnimateUnmatched: true,
            });

            await revealInstance.current.initialize();
            
            revealInstance.current.on('slidechanged', (event: Event) => {
                const slideEvent = event as unknown as SlideChangeEvent;
                if (onSlideChangeRef.current) {
                    onSlideChangeRef.current(slideEvent.indexh, slideEvent.indexv);
                }
            });

            setIsLoaded(true);
        };

        initReveal();

        return () => {
            mounted = false;
            try {
                if (revealInstance.current) {
                    revealInstance.current.destroy();
                    revealInstance.current = null;
                }
            } catch (e) {
                console.warn('Reveal destruction error', e);
            }
        };
    }, [fullscreen]);

    // Sync layout when markdown changes
    useEffect(() => {
        if (isLoaded && revealInstance.current) {
            try {
                revealInstance.current.sync();
                revealInstance.current.layout();
            } catch (e) {
                console.warn('Reveal sync error', e);
            }
        }
    }, [markdown, isLoaded]);

    return (
        <div ref={containerRef} className={`${styles.deckContainer} ${fullscreen ? styles.fullscreen : ''}`}>
            <div className="reveal" ref={deckRef}>
                <div className="slides">
                    <section 
                        data-markdown=""
                        data-separator="^\n---\n" 
                        data-separator-vertical="^\n--\n" 
                        data-separator-notes="^>\s*备注[：:]"
                    >
                        <textarea data-template defaultValue={markdown} />
                    </section>
                </div>
            </div>
        </div>
    );
}
