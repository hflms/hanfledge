import { render, screen, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import SessionPage from './page';

const mockRouter = { push: vi.fn(), back: vi.fn() };
vi.mock('next/navigation', () => ({
    useParams: () => ({ id: '1' }),
    useRouter: () => mockRouter,
}));

vi.mock('next/link', () => ({
    default: ({ children }: { children: React.ReactNode }) => <a>{children}</a>,
}));

vi.mock('next/dynamic', () => ({
    default: (fn: () => Promise<{ default: React.ComponentType }>) => {
        return function MockDynamicComponent() {
            return <div data-testid="mock-dynamic">Dynamic</div>;
        };
    },
}));

vi.mock('@/lib/api', () => ({
    getSession: vi.fn(),
    getSystemConfig: vi.fn().mockResolvedValue({}),
}));

const mockPlugins: unknown[] = [];
vi.mock('@/lib/plugin/PluginRegistry', () => ({
    usePluginRegistry: () => mockPlugins,
}));

vi.mock('@/lib/plugin/SkillRendererPlugins', () => ({
    useBuiltinSkillRenderers: () => ({}),
}));

vi.mock('@/lib/cache/indexedDBCache', () => ({
    getCachedResponse: vi.fn().mockResolvedValue(null),
    setCachedResponse: vi.fn(),
    purgeExpiredEntries: vi.fn().mockResolvedValue(0),
}));

const mockToast = vi.fn();
vi.mock('@/components/Toast', () => ({
    useToast: () => ({ toast: mockToast }),
    ToastProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('./hooks/useSessionWebSocket', () => ({
    useSessionWebSocket: vi.fn(),
}));

interface MockMessage {
    id: string;
    content: string;
}

vi.mock('./components/MessageList', () => ({
    default: ({ messages, streamingContent }: { messages: MockMessage[]; streamingContent?: string }) => (
        <div data-testid="mock-message-list">
            {messages.map((m) => (
                <div key={m.id}>{m.content}</div>
            ))}
            {streamingContent && <div>{streamingContent}</div>}
        </div>
    )
}));

vi.mock('./components/ScaffoldPanel', () => ({
    default: () => <div data-testid="mock-scaffold-panel" />,
    SCAFFOLD_LABELS: {},
    SCAFFOLD_DESCRIPTIONS: {},
}));

vi.mock('./components/SessionInput', () => ({
    default: ({ input, setInput, onSend }: { input: string; setInput: (v: string) => void; onSend: () => void }) => (
        <div data-testid="mock-session-input">
            <input 
                data-testid="input-field"
                value={input} 
                onChange={(e) => setInput(e.target.value)} 
            />
            <button data-testid="send-btn" onClick={() => onSend()}>Send</button>
        </div>
    ),
}));

// Setup mocks and clear between tests
import { getSession } from '@/lib/api';
import { useSessionWebSocket } from './hooks/useSessionWebSocket';

interface MockAgentChannel {
    send: ReturnType<typeof vi.fn>;
    onMessage: ReturnType<typeof vi.fn>;
    onClose: ReturnType<typeof vi.fn>;
    close: ReturnType<typeof vi.fn>;
}

describe('SessionPage', () => {
    let mockAgentChannel: MockAgentChannel;

    beforeEach(() => {
        mockAgentChannel = {
            send: vi.fn(),
            onMessage: vi.fn(() => vi.fn()),
            onClose: vi.fn(() => vi.fn()),
            close: vi.fn(),
        };

        (getSession as ReturnType<typeof vi.fn>).mockResolvedValue({
            session: {
                id: 1,
                course_id: 1,
                knowledge_point_id: 101,
                status: 'active',
                started_at: new Date().toISOString(),
                scaffold_level: 'low',
                course: { id: 1, title: 'Test Course', description: '' },
                knowledge_point: { id: 101, title: 'Test Point', chapter_title: 'Ch 1', difficulty: 1 },
            },
            interactions: []
        });

        (useSessionWebSocket as ReturnType<typeof vi.fn>).mockReturnValue({
            wsStatus: 'connected',
            reconnectCount: 0,
            agentChannel: mockAgentChannel,
        });
    });

    afterEach(() => {
        vi.clearAllMocks();
    });

    it('should load session and display initial UI', async () => {
        render(<SessionPage />);

        // Wait for session data to load
        await screen.findByText('AI 学习对话');

        expect(getSession).toHaveBeenCalledWith(1);
        expect(screen.getByTestId('mock-message-list')).toBeInTheDocument();
        expect(screen.getByTestId('mock-session-input')).toBeInTheDocument();
    });

    it('should handle sending a message', async () => {
        render(<SessionPage />);

        await screen.findByText('AI 学习对话');

        const input = screen.getByTestId('input-field');
        const sendBtn = screen.getByTestId('send-btn');

        await userEvent.type(input, 'Hello AI');

        expect(input).toHaveValue('Hello AI');

        await userEvent.click(sendBtn);

        // The user's message should be added to the UI
        await screen.findByText('Hello AI');
        expect(screen.getByText('Hello AI')).toBeInTheDocument();
        
        // Input should be cleared
        expect(input).toHaveValue('');

        // agentChannel should have been called
        expect(mockAgentChannel.send).toHaveBeenCalled();
        const callArg = JSON.parse(mockAgentChannel.send.mock.calls[0][0]);
        expect(callArg.event).toBe('user_message');
        expect(callArg.payload.text).toBe('Hello AI');
    });

    it('should handle incoming websocket events (stream_start, token, stream_end)', async () => {
        type WSEventHandler = (event: { event: string; payload?: unknown; timestamp: number }) => void;
        let capturedOnEvent: WSEventHandler | undefined;
        
        (useSessionWebSocket as ReturnType<typeof vi.fn>).mockImplementation(({ onEvent }: { onEvent: WSEventHandler }) => {
            capturedOnEvent = onEvent;
            return {
                wsStatus: 'connected',
                reconnectCount: 0,
                agentChannel: mockAgentChannel,
            };
        });

        render(<SessionPage />);
        await screen.findByText('AI 学习对话');

        // Send a message first so pendingQuestionRef is set
        const input = screen.getByTestId('input-field');
        const sendBtn = screen.getByTestId('send-btn');

        await userEvent.type(input, 'Testing streaming');
        await userEvent.click(sendBtn);

        // Simulate incoming stream events
        await act(async () => {
            capturedOnEvent?.({ event: 'agent_thinking', payload: { status: 'thinking' }, timestamp: Date.now() });
        });

        await act(async () => {
            capturedOnEvent?.({ event: 'token_delta', payload: { text: 'Response text' }, timestamp: Date.now() });
        });

        await act(async () => {
            capturedOnEvent?.({ event: 'turn_complete', payload: {}, timestamp: Date.now() });
        });

        // The final message should be appended to the list
        expect(screen.getByText('Response text')).toBeInTheDocument();
    });
});
