import { useCallback, useEffect, useRef, useState } from 'react';
import type { Briefing, InboundFrame, Message } from '../types/message';

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
const RECONNECT_DELAYS = [1000, 2000, 4000, 8000, 16000];

export function useWebSocket(url: string, options: UseWebSocketOptions = {}): UseWebSocketReturn {
  const [messages, setMessages] = useState<Message[]>([]);
  const [connectionStatus, setConnectionStatus] = useState<ConnectionStatus>('connecting');
  const wsRef = useRef<WebSocket | null>(null);
  const attemptRef = useRef(0);
  const mountedRef = useRef(true);

  // Held in a ref so a changing callback identity never tears down the socket.
  const onBriefingRef = useRef(options.onBriefing);
  onBriefingRef.current = options.onBriefing;

  const connect = useCallback(() => {
    if (!mountedRef.current) return;
    if (!url) return;

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
      // writePump can batch multiple JSON objects separated by newlines
      const lines = (event.data as string).split('\n').filter(Boolean);
      const parsed: Message[] = [];
      for (const line of lines) {
        try {
          const frame = JSON.parse(line) as InboundFrame;
          if (frame.type === 'briefing.ready') {
            onBriefingRef.current?.(frame.briefing);
          } else if (
            frame.type === 'chat' ||
            frame.type === 'system' ||
            frame.type === 'join' ||
            frame.type === 'leave'
          ) {
            parsed.push(frame);
          }
        } catch {
          // ignore malformed lines
        }
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
        setTimeout(connect, RECONNECT_DELAYS[attempt]);
      } else {
        setConnectionStatus('failed');
      }
    };

    ws.onerror = () => {
      // onclose fires after onerror; reconnect logic is handled there
      setConnectionStatus('disconnected');
    };
  }, [url]);

  useEffect(() => {
    mountedRef.current = true;
    if (url) connect();
    else setConnectionStatus('disconnected');
    return () => {
      mountedRef.current = false;
      wsRef.current?.close();
    };
  }, [connect]);

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
