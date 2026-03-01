import { renderHook, act } from '@testing-library/react';
import { vi, describe, it, expect, beforeEach, afterEach } from 'vitest';
import { useSessionWebSocket } from './useSessionWebSocket';
import { createWSUrl } from '@/lib/api';

// Mock the API module
vi.mock('@/lib/api', () => ({
    createWSUrl: vi.fn((id) => `ws://localhost/mock-ws/${id}`),
}));

describe('useSessionWebSocket', () => {
    let mockWebSocket: any;

    beforeEach(() => {
        // Mock global WebSocket
        mockWebSocket = {
            send: vi.fn(),
            close: vi.fn(),
            addEventListener: vi.fn(),
            removeEventListener: vi.fn(),
            readyState: 1, // WebSocket.OPEN is 1
        };

        const MockWebSocketConstructor = vi.fn(function() {
            return mockWebSocket;
        });
        MockWebSocketConstructor.OPEN = 1;
        MockWebSocketConstructor.CONNECTING = 0;
        MockWebSocketConstructor.CLOSING = 2;
        MockWebSocketConstructor.CLOSED = 3;

        global.WebSocket = MockWebSocketConstructor as any;

        vi.useFakeTimers();
    });

    afterEach(() => {
        vi.restoreAllMocks();
        vi.clearAllMocks();
    });

    it('should not connect if sessionStatus is not active', () => {
        const { result } = renderHook(() =>
            useSessionWebSocket({
                sessionId: 1,
                sessionStatus: 'completed',
                onEvent: vi.fn(),
            })
        );

        expect(global.WebSocket).not.toHaveBeenCalled();
        expect(result.current.wsStatus).toBe('disconnected');
    });

    it('should connect when sessionStatus is active', () => {
        const { result } = renderHook(() =>
            useSessionWebSocket({
                sessionId: 1,
                sessionStatus: 'active',
                onEvent: vi.fn(),
            })
        );

        expect(global.WebSocket).toHaveBeenCalledWith('ws://localhost/mock-ws/1');
        expect(result.current.wsStatus).toBe('connecting');

        // Simulate open event
        act(() => {
            mockWebSocket.onopen();
        });

        expect(result.current.wsStatus).toBe('connected');
    });

    it('should call onEvent and agentChannel handlers on message', () => {
        const mockOnEvent = vi.fn();
        const { result } = renderHook(() =>
            useSessionWebSocket({
                sessionId: 1,
                sessionStatus: 'active',
                onEvent: mockOnEvent,
            })
        );

        const mockPluginHandler = vi.fn();
        
        act(() => {
            mockWebSocket.onopen();
            result.current.agentChannel.onMessage(mockPluginHandler);
        });

        const mockData = JSON.stringify({ event: 'test_event', payload: {} });

        act(() => {
            mockWebSocket.onmessage({ data: mockData });
        });

        expect(mockOnEvent).toHaveBeenCalledWith({ event: 'test_event', payload: {} });
        expect(mockPluginHandler).toHaveBeenCalledWith(mockData);
    });

    it('should be able to send messages via agentChannel when connected', () => {
        const { result } = renderHook(() =>
            useSessionWebSocket({
                sessionId: 1,
                sessionStatus: 'active',
                onEvent: vi.fn(),
            })
        );

        act(() => {
            mockWebSocket.onopen();
        });

        const msg = JSON.stringify({ event: 'ping' });
        act(() => {
            result.current.agentChannel.send(msg);
        });

        expect(mockWebSocket.send).toHaveBeenCalledWith(msg);
    });

    it('should attempt reconnection on close (intentionalClose = false)', () => {
        const { result } = renderHook(() =>
            useSessionWebSocket({
                sessionId: 1,
                sessionStatus: 'active',
                onEvent: vi.fn(),
            })
        );

        act(() => {
            mockWebSocket.onopen();
        });

        // First disconnect
        act(() => {
            mockWebSocket.onclose({ code: 1006, reason: 'abnormal' });
        });

        expect(result.current.wsStatus).toBe('reconnecting');
        expect(result.current.reconnectCount).toBe(1);

        // Advance timers to trigger reconnect
        act(() => {
            vi.advanceTimersByTime(1000); // WS_RECONNECT_BASE_DELAY
        });

        // WebSocket should be instantiated again
        expect(global.WebSocket).toHaveBeenCalledTimes(2);
    });
});
