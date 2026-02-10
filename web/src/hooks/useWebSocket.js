import { useState, useEffect, useCallback, useRef } from 'react';

const WS_URL = process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws';

export function useWebSocket(onMessage) {
  const [connected, setConnected] = useState(false);
  const wsRef = useRef(null);
  const reconnectTimer = useRef(null);

  const connect = useCallback(() => {
    try {
      const ws = new WebSocket(WS_URL);
      wsRef.current = ws;

      ws.onopen = () => setConnected(true);

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessage(data);
        } catch (err) {
          console.error('Failed to parse WebSocket message:', err);
        }
      };

      ws.onclose = () => {
        setConnected(false);
        reconnectTimer.current = setTimeout(connect, 3000);
      };

      ws.onerror = () => ws.close();
    } catch (err) {
      console.error('WebSocket connection failed:', err);
      reconnectTimer.current = setTimeout(connect, 3000);
    }
  }, [onMessage]);

  useEffect(() => {
    connect();
    return () => {
      if (wsRef.current) wsRef.current.close();
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current);
    };
  }, [connect]);

  return connected;
}
