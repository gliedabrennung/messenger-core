import {
  createContext,
  useContext,
  useEffect,
  useRef,
  useCallback,
  useState,
  type ReactNode,
  type FC,
  createElement,
} from 'react';
import { api } from '@/api';
import { useAuthStore } from '@/store/authStore';
import { useChatStore } from '@/store/chatStore';
import type { ConnectionStatus } from '@/types';

const MAX_RECONNECT_DELAY = 30_000;
const BASE_DELAY = 1_000;

const FAILED_HANDSHAKES_BEFORE_SESSION_CHECK = 3;

interface WebSocketContextValue {
  sendMessage: (toId: number, message: string, clientId: string) => boolean;
  status: ConnectionStatus;
}

const WebSocketContext = createContext<WebSocketContextValue>({
  sendMessage: () => false,
  status: 'disconnected',
});

export const useWebSocket = () => useContext(WebSocketContext);

export const WebSocketProvider: FC<{ children: ReactNode }> = ({ children }) => {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  const userId = useAuthStore((s) => s.user?.id);
  const wsRef = useRef<WebSocket | null>(null);
  const retriesRef = useRef(0);
  const failedHandshakesRef = useRef(0);
  const mountedRef = useRef(true);
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');

  useEffect(() => {
    mountedRef.current = true;
    if (!isAuthenticated || !userId) {
      setStatus('disconnected');
      return;
    }

    const connect = () => {
      if (!mountedRef.current) return;
      if (wsRef.current?.readyState === WebSocket.OPEN || wsRef.current?.readyState === WebSocket.CONNECTING) {
        return;
      }

      setStatus('connecting');

      let opened = false;
      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
      const ws = new WebSocket(`${protocol}//${location.host}/ws`);
      wsRef.current = ws;

      ws.onopen = () => {
        if (!mountedRef.current) { ws.close(); return; }
        opened = true;
        retriesRef.current = 0;
        failedHandshakesRef.current = 0;
        setStatus('connected');
      };

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          const chat = useChatStore.getState();

          if (data.type === 'ack') {
            if (!data.client_id || !data.to) return;
            chat.resolveMessage(data.to, data.client_id, {
              message_id: data.message_id,
              created_at: data.created_at,
              isPending: false,
              failed: false,
            });
            return;
          }

          if (data.type === 'error') {
            if (data.client_id && data.to) {
              chat.resolveMessage(data.to, data.client_id, {
                isPending: false,
                failed: true,
              });
            }
            console.warn('websocket error frame:', data.code, data.message);
            return;
          }

          if (!data.from || !data.message) return;

          const myId = useAuthStore.getState().user?.id;

          const partnerId = data.from === myId ? data.to : data.from;
          if (!partnerId) return;

          chat.addMessage(partnerId, {
            message_id: data.message_id,
            from_id: data.from,
            to_id: data.to,
            content: data.message,
            created_at: data.created_at ?? new Date().toISOString(),
          });
        } catch {
          /* malformed message */
        }
      };

      ws.onclose = () => {
        wsRef.current = null;
        if (!mountedRef.current) return;
        setStatus('disconnected');

        if (!opened) {
          failedHandshakesRef.current++;
          if (failedHandshakesRef.current === FAILED_HANDSHAKES_BEFORE_SESSION_CHECK) {
            api.get('/users/me').catch(() => {});
          }
        }

        const delay = Math.min(BASE_DELAY * 2 ** retriesRef.current, MAX_RECONNECT_DELAY);
        retriesRef.current++;
        reconnectTimerRef.current = setTimeout(connect, delay);
      };

      ws.onerror = () => {};
    };

    connect();

    return () => {
      mountedRef.current = false;
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      if (wsRef.current) {
        wsRef.current.onclose = null;
        wsRef.current.onerror = null;
        wsRef.current.close();
        wsRef.current = null;
      }
      setStatus('disconnected');
    };
  }, [isAuthenticated, userId]);

  const sendMessage = useCallback((toId: number, message: string, clientId: string) => {
    if (wsRef.current?.readyState !== WebSocket.OPEN) return false;
    wsRef.current.send(JSON.stringify({ to: toId, message, client_id: clientId }));
    return true;
  }, []);

  return createElement(WebSocketContext.Provider, { value: { sendMessage, status } }, children);
};
