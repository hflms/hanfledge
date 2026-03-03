import { useCallback, useEffect, useRef, useState, useMemo } from 'react';
import { createWSUrl, type WSEvent } from '@/lib/api';
import type { AgentWebSocketChannel } from '@/lib/plugin/types';

// -- Reconnect Constants -----------------------------------------

const WS_RECONNECT_BASE_DELAY = 1000;  // 1s initial delay
const WS_RECONNECT_MAX_DELAY = 30000;  // 30s max delay
const WS_RECONNECT_MAX_RETRIES = 8;
const WS_PING_INTERVAL = 30000;        // 30s heartbeat ping

export type WSStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected';

interface UseSessionWebSocketOptions {
    sessionId: number;
    sessionStatus: string | undefined;
    /** Called for each incoming WSEvent when no plugin renderer is active */
    onEvent: (event: WSEvent) => void;
}

interface UseSessionWebSocketReturn {
    wsRef: React.RefObject<WebSocket | null>;
    wsStatus: WSStatus;
    reconnectCount: number;
    agentChannel: AgentWebSocketChannel;
}

/**
 * Manages the WebSocket lifecycle for a student session:
 *  - connects when session is active
 *  - heartbeat pings
 *  - exponential-backoff reconnection
 *  - exposes an AgentWebSocketChannel for plugin renderers
 */
export function useSessionWebSocket({
    sessionId,
    sessionStatus,
    onEvent,
}: UseSessionWebSocketOptions): UseSessionWebSocketReturn {
    const [wsStatus, setWsStatus] = useState<WSStatus>('disconnected');

    const wsRef = useRef<WebSocket | null>(null);
    const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const pingTimerRef = useRef<ReturnType<typeof setInterval> | null>(null);
    const reconnectCountRef = useRef(0);
    const intentionalCloseRef = useRef(false);

    // Plugin handlers
    const wsMessageHandlersRef = useRef<Array<(data: string) => void>>([]);
    const wsCloseHandlersRef = useRef<Array<() => void>>([]);

    // Store onEvent in a ref so it doesn't trigger reconnection
    const onEventRef = useRef(onEvent);
    onEventRef.current = onEvent;

    const agentChannel = useMemo<AgentWebSocketChannel>(() => ({
        send: (message: string) => {
            console.log('[WS DEBUG] 发送消息:', message.substring(0, 500));
            if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
                wsRef.current.send(message);
            } else {
                console.warn('[WS DEBUG] WebSocket 未就绪, readyState:', wsRef.current?.readyState);
            }
        },
        onMessage: (handler: (data: string) => void) => {
            wsMessageHandlersRef.current.push(handler);
            return () => {
                wsMessageHandlersRef.current = wsMessageHandlersRef.current.filter(h => h !== handler);
            };
        },
        onClose: (handler: () => void) => {
            wsCloseHandlersRef.current.push(handler);
            return () => {
                wsCloseHandlersRef.current = wsCloseHandlersRef.current.filter(h => h !== handler);
            };
        },
        close: () => {
            intentionalCloseRef.current = true;
            wsRef.current?.close();
        },
    }), []);

    const connectWebSocket = useCallback(() => {
        if (sessionStatus !== 'active') return;
        if (wsRef.current) {
            const state = wsRef.current.readyState;
            if (state === WebSocket.OPEN || state === WebSocket.CONNECTING) {
                console.warn('[WS DEBUG] WebSocket 已存在，跳过重复连接');
                return;
            }
            if (state === WebSocket.CLOSING) {
                wsRef.current.close();
            }
        }

        const wsUrl = createWSUrl(sessionId);
        console.log('[WS DEBUG] 连接 URL:', wsUrl);
        setWsStatus(reconnectCountRef.current > 0 ? 'reconnecting' : 'connecting');

        const ws = new WebSocket(wsUrl);
        wsRef.current = ws;

        ws.onopen = () => {
            console.log('[WS] Connected to session', sessionId);
            setWsStatus('connected');
            reconnectCountRef.current = 0;

            pingTimerRef.current = setInterval(() => {
                if (ws.readyState === WebSocket.OPEN) {
                    ws.send(JSON.stringify({ event: 'ping', payload: {}, timestamp: Math.floor(Date.now() / 1000) }));
                }
            }, WS_PING_INTERVAL);
        };

        ws.onmessage = (event) => {
            try {
                const wsEvent: WSEvent = JSON.parse(event.data);
                console.log('[WS DEBUG] 收到事件:', wsEvent.event, JSON.stringify(wsEvent.payload).substring(0, 200));
                for (const handler of wsMessageHandlersRef.current) {
                    handler(event.data);
                }
                onEventRef.current(wsEvent);
            } catch (err) {
                console.error('[WS] Parse error:', err);
            }
        };

        ws.onclose = (event) => {
            console.log('[WS] Disconnected:', event.code, event.reason);
            wsRef.current = null;

            for (const handler of wsCloseHandlersRef.current) {
                handler();
            }

            if (pingTimerRef.current) {
                clearInterval(pingTimerRef.current);
                pingTimerRef.current = null;
            }

            if (!intentionalCloseRef.current && reconnectCountRef.current < WS_RECONNECT_MAX_RETRIES) {
                const delay = Math.min(
                    WS_RECONNECT_BASE_DELAY * Math.pow(2, reconnectCountRef.current),
                    WS_RECONNECT_MAX_DELAY
                );
                reconnectCountRef.current += 1;
                setWsStatus('reconnecting');
                console.log(`[WS] Reconnecting in ${delay}ms (attempt ${reconnectCountRef.current}/${WS_RECONNECT_MAX_RETRIES})`);
                reconnectTimerRef.current = setTimeout(connectWebSocket, delay);
            } else {
                setWsStatus('disconnected');
                if (reconnectCountRef.current >= WS_RECONNECT_MAX_RETRIES) {
                    console.warn('[WS] Max reconnect attempts reached');
                }
            }
        };

        ws.onerror = (err) => {
            console.warn('[WS] Connection error:', err);
        };
    }, [sessionId, sessionStatus]);

    useEffect(() => {
        if (sessionStatus !== 'active') return;

        intentionalCloseRef.current = false;
        // DO NOT clear handlers here. Child components register their handlers
        // in their own useEffects which run BEFORE this parent useEffect.
        // Clearing them here wipes out their subscriptions.
        connectWebSocket();

        return () => {
            intentionalCloseRef.current = true;
            if (reconnectTimerRef.current) {
                clearTimeout(reconnectTimerRef.current);
                reconnectTimerRef.current = null;
            }
            if (pingTimerRef.current) {
                clearInterval(pingTimerRef.current);
                pingTimerRef.current = null;
            }
            if (wsRef.current) {
                wsRef.current.close();
                wsRef.current = null;
            }
        };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [sessionId, sessionStatus]);

    return {
        wsRef,
        wsStatus,
        reconnectCount: reconnectCountRef.current,
        agentChannel,
    };
}
