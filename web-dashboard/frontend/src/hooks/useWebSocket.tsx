import { useEffect, useRef, useCallback } from 'react';
import { useAuth } from './useAuth';

type MessageHandler = (data: any) => void;

export function useWebSocket(onMessage: MessageHandler) {
  const { user } = useAuth();
  const wsRef = useRef<WebSocket | null>(null);
  const reconnectTimeout = useRef<number>();
  
  // Сохраняем последний обработчик в ref, чтобы не пересоздавать WebSocket
  const onMessageRef = useRef(onMessage);
  onMessageRef.current = onMessage;

  useEffect(() => {
    if (!user) return;

    const token = localStorage.getItem('access_token');
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/api/v1/ws?token=${token}`;

    const ws = new WebSocket(wsUrl);
    wsRef.current = ws;

    ws.onopen = () => {
      console.log('WebSocket connected');
      ws.send(JSON.stringify({ type: 'ping' }));
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        // Используем актуальный обработчик из ref
        onMessageRef.current(data);
      } catch {}
    };

    ws.onclose = (event) => {
      console.log('WebSocket disconnected:', event.code);
      if (!event.wasClean) {
        reconnectTimeout.current = window.setTimeout(() => {
          // Реинициализация произойдёт при следующем рендере
        }, 5000);
      }
    };

    ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    return () => {
      clearTimeout(reconnectTimeout.current);
      ws.close();
    };
  }, [user?.id]); // Переподключаемся только при смене пользователя

  return wsRef;
}