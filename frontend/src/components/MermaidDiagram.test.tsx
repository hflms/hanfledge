import React from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import MermaidDiagram from './MermaidDiagram';
import DOMPurify from 'dompurify';
import mermaid from 'mermaid';

// Mock mermaid module
vi.mock('mermaid', () => ({
    default: {
        initialize: vi.fn(),
        render: vi.fn(),
    },
}));

// Mock DOMPurify to spy on sanitize calls without changing its behavior
vi.mock('dompurify', async (importOriginal) => {
    const actual = await importOriginal();
    return {
        default: {
            // @ts-expect-error - overriding default mock behavior for actual
            ...actual.default,
            // @ts-expect-error - overriding default mock behavior for actual
            sanitize: vi.fn(actual.default.sanitize),
        },
    };
});

// Mock generateId to avoid random IDs
vi.mock('@/lib/utils', () => ({
    generateId: vi.fn(() => 'mermaid-test-id'),
}));

describe('MermaidDiagram Component', () => {
    it('should initialize mermaid with strict security level', async () => {
        (mermaid.render as import('vitest').Mock).mockResolvedValue({ svg: '<svg>test</svg>' });

        await waitFor(() => {
            render(<MermaidDiagram chart="graph TD; A-->B;" />);
        });

        expect(mermaid.initialize).toHaveBeenCalledWith(
            expect.objectContaining({
                securityLevel: 'strict',
            })
        );
    });

    it('should sanitize mermaid svg output with DOMPurify', async () => {
        const maliciousSvg = '<svg><script>alert("xss")</script><g>test</g></svg>';
        (mermaid.render as import('vitest').Mock).mockResolvedValue({ svg: maliciousSvg });

        await waitFor(() => {
            render(<MermaidDiagram chart="graph TD; A-->B;" />);
        });

        await waitFor(() => {
            expect(DOMPurify.sanitize).toHaveBeenCalledWith(
                maliciousSvg,
                expect.objectContaining({
                    USE_PROFILES: { svg: true },
                })
            );
        });

        // The script tag should be removed by DOMPurify
        // Because of dangerouslySetInnerHTML, testing the exact output is tricky without actually querying DOM
        await waitFor(() => {
            const containers = screen.queryAllByText((content, element) => {
                return element?.tagName.toLowerCase() === 'div' && element?.innerHTML.includes('<svg><g>test</g></svg>');
            });
            expect(containers.length).toBeGreaterThan(0);
        });
    });
});
