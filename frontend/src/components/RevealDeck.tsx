'use client';

import React, { useEffect, useRef, useState } from 'react';
import 'reveal.js/dist/reveal.css';
import 'reveal.js/dist/theme/white.css';
import styles from './RevealDeck.module.css';

interface RevealDeckProps {
    markdown: string;
    onSlideChange?: (indexh: number, indexv: number) => void;
}

export default function RevealDeck({ markdown, onSlideChange }: RevealDeckProps) {
    const deckRef = useRef<HTMLDivElement>(null);
    const revealInstance = useRef<any>(null);
    const [isLoaded, setIsLoaded] = useState(false);
    const onSlideChangeRef = useRef(onSlideChange);

    useEffect(() => {
        onSlideChangeRef.current = onSlideChange;
    }, [onSlideChange]);

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
                embedded: true,
                width: 960,
                height: 700,
                margin: 0.1,
            });

            await revealInstance.current.initialize();
            
            revealInstance.current.on('slidechanged', (event: any) => {
                if (onSlideChangeRef.current) {
                    onSlideChangeRef.current(event.indexh, event.indexv);
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
    }, []);

    // Handle updates to markdown content
    useEffect(() => {
        if (isLoaded && revealInstance.current) {
            try {
                // If the markdown content changes, we might need to recreate the textarea content
                // and call layout syncs. 
                // But for simplicity, we can just remount RevealDeck if markdown changes drastically.
            } catch (e) {
                console.warn('Reveal sync error', e);
            }
        }
    }, [markdown, isLoaded]);

    return (
        <div className={styles.deckContainer}>
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
