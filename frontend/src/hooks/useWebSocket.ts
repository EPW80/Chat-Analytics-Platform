import { useCallback, useEffect, useRef, useState } from 'react';
import type { Briefing, Message } from '../types/message';
import { parseInboundFrames } from '../lib/wsFrames';

export type ConnectionStatus = 'connecting' | 'connected' | 'disconnected' | 'failed';

interface UseWebSocketOptions {
  /** Called for each `briefing.ready` frame. Briefings are not chat, so they
   *  never enter `messages` — they go straight to the caller. */
  onBriefing?: (briefing: Briefing) => void;
}

interface UseWebSocketReturn {
  messages: Message[];
  sendMessage: (content: string) => void;
  isConnected: boolean;
  connectionStatus: ConnectionStatus;
}

const MAX_RECONNECT_ATTEMPTS = 5;
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000] as const;

export function useWebSocket(url: string, options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting');
  const wsRef = useRef<WebSocket | null>(null);
  const attemptRef = useRef(0);
  const mountedRef = useRef(true);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Held in a ref so a changing callback identity never tears down the socket.
  const onBriefingRef = useRef(options.onBriefing);
  onBriefingRef.current = options.onBriefing;

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current !== null) {
      clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = null;
    }
  }, []);

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    if (!url) return;

    clearReconnectTimer();
    setConnectionStatus('connecting');
    const ws = new WebSocket(url);
    wsRef.current = ws;

    ws.onopen = () => {
      if (!mountedRef.current) return;
      attemptRef.current = 0;
      setConnectionStatus('connected');
    };

    ws.onmessage = (event) => {
      if (!mountedRef.current) return;
      const { messages: parsed, briefings } = parseInboundFrames(event.data as string);
      for (const briefing of briefings) {
        onBriefingRef.current?.(briefing);
      }
      if (parsed.length > 0) {
        setMessages((prev) => [...prev, ...parsed].slice(-500)); // keep last 500
      }
    };

    ws.onclose = () => {
      if (!mountedRef.current) return;
      wsRef.current = null;
      const attempt = attemptRef.current;
      if (attempt < MAX_RECONNECT_ATTEMPTS) {
        setConnectionStatus('connecting');
        attemptRef.current = attempt + 1;
        const delay = RECONNECT_DELAYS[attempt] ?? RECONNECT_DELAYS[RECONNECT_DELAYS.length - 1];
        reconnectTimerRef.current = setTimeout(connect, delay);
      } else {
        setConnectionStatus('failed');
      }
    };

    ws.onerror = () => {
      // onclose fires after onerror; reconnect logic is handled there
      setConnectionStatus('disconnected');
    };
  }, [url, clearReconnectTimer]);

  useEffect(() => {
    mountedRef.current = true;
    if (url) connect();
    else setConnectionStatus('disconnected');
    return () => {
      mountedRef.current = false;
      clearReconnectTimer();
      wsRef.current?.close();
    };
  }, [connect, clearReconnectTimer, url]);

  const sendMessage = useCallback((content: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify({ type: 'chat', content }));
    }
  }, []);

  return {
    messages,
    sendMessage,
    isConnected: connectionStatus === 'connected',
    connectionStatus,
  };
}
