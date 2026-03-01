import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import MessageList, { type ChatMessage } from './MessageList';

// Mock next/dynamic components to render synchronously in tests
vi.mock('next/dynamic', () => ({
    default: (dynamicImport: any) => {
        // Return a dummy component for MarkdownRenderer
        return function MockMarkdown({ content }: { content: string }) {
            return <div data-testid="markdown-mock">{content}</div>;
        };
    },
}));

// Mock scrollIntoView to avoid errors in JSDOM
window.HTMLElement.prototype.scrollIntoView = vi.fn();

describe('MessageList Component', () => {
    it('should render empty state when no messages', () => {
        render(<MessageList messages={[]} streamingContent="" thinkingStatus={null} />);
        expect(screen.getByText('发送消息开始学习对话')).toBeInTheDocument();
    });

    it('should render student and coach messages', () => {
        const messages: ChatMessage[] = [
            { id: '1', role: 'student', content: 'Hello AI', timestamp: 123 },
            { id: '2', role: 'coach', content: 'Hello Student', timestamp: 124 },
        ];

        render(<MessageList messages={messages} streamingContent="" thinkingStatus={null} />);
        
        // Student message
        expect(screen.getByText('我')).toBeInTheDocument();
        expect(screen.getByText('Hello AI')).toBeInTheDocument();
        
        // Coach message
        expect(screen.getByText('AI 导师')).toBeInTheDocument();
        expect(screen.getByTestId('markdown-mock')).toHaveTextContent('Hello Student');
    });

    it('should render streaming content', () => {
        render(
            <MessageList 
                messages={[]} 
                streamingContent="Streaming text..." 
                thinkingStatus={null} 
            />
        );
        
        expect(screen.getByText('AI 导师')).toBeInTheDocument();
        expect(screen.getByTestId('markdown-mock')).toHaveTextContent('Streaming text...');
    });

    it('should render thinking status', () => {
        render(
            <MessageList 
                messages={[]} 
                streamingContent="" 
                thinkingStatus="思考中..." 
            />
        );
        
        expect(screen.getByText('思考中...')).toBeInTheDocument();
    });

    it('should call scrollIntoView on update', () => {
        // Reset mock to ensure we start counting from 0
        (window.HTMLElement.prototype.scrollIntoView as any).mockClear();

        const { rerender } = render(
            <MessageList messages={[]} streamingContent="" thinkingStatus={null} />
        );
        
        // First render triggers scroll
        expect(window.HTMLElement.prototype.scrollIntoView).toHaveBeenCalledTimes(1);

        // Update props
        rerender(
            <MessageList 
                messages={[{ id: '1', role: 'student', content: 'update', timestamp: 1 }]} 
                streamingContent="" 
                thinkingStatus={null} 
            />
        );
        
        // Should scroll again
        expect(window.HTMLElement.prototype.scrollIntoView).toHaveBeenCalledTimes(2);
    });
});
