'use client';

import React, { useEffect, useRef, useState } from 'react';
import mermaid from 'mermaid';

interface MermaidDiagramProps {
    chart: string;
}

export default function MermaidDiagram({ chart }: MermaidDiagramProps) {
    const containerRef = useRef<HTMLDivElement>(null);
    const [svg, setSvg] = useState<string>('');
    const [error, setError] = useState<string>('');

    useEffect(() => {
        mermaid.initialize({
            startOnLoad: false,
            theme: 'default',
            securityLevel: 'loose',
            fontFamily: 'inherit',
        });

        let isMounted = true;

        const renderChart = async () => {
            try {
                if (!chart) return;
                const id = `mermaid-${Math.random().toString(36).slice(2, 9)}`;
                const { svg: renderedSvg } = await mermaid.render(id, chart);
                if (isMounted) {
                    setSvg(renderedSvg);
                    setError('');
                }
            } catch (err: any) {
                if (isMounted) {
                    setError(err?.message || 'Failed to render diagram');
                }
            }
        };

        renderChart();

        return () => {
            isMounted = false;
        };
    }, [chart]);

    if (error) {
        return (
            <div style={{ padding: '1rem', border: '1px solid #ff4d4f', color: '#ff4d4f', borderRadius: '4px', margin: '1rem 0' }}>
                <p><strong>图表渲染失败:</strong></p>
                <pre style={{ whiteSpace: 'pre-wrap', fontSize: '0.875rem' }}>{error}</pre>
            </div>
        );
    }

    return (
        <div 
            ref={containerRef}
            dangerouslySetInnerHTML={{ __html: svg }}
            style={{ 
                display: 'flex', 
                justifyContent: 'center', 
                margin: '1.5rem 0',
                overflowX: 'auto',
                background: '#f8f9fa',
                padding: '1rem',
                borderRadius: '8px'
            }}
        />
    );
}
