/**
 * ws.ts — Lightweight WebSocket client for Pi-Go mobile
 *
 * RN Best Practices:
 * - js-memory-leaks: reconnect timer cleaned up on disconnect
 * - Exponential backoff (3s → 6s → 12s → max 30s) to avoid hammering
 */

type WsListener = (data: any) => void;

class WsClient {
  private ws: WebSocket | null = null;
  private listeners = new Map<string, Set<WsListener>>();
  private url = '';
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private static MAX_RECONNECT_MS = 30_000;

  connect(url: string): void {
    this.url = url.replace(/^http/, 'ws') + '/ws';
    this.reconnectAttempts = 0;
    this.doConnect();
  }

  private doConnect(): void {
    try {
      this.ws = new WebSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.emit('connected', {});
    };

    this.ws.onmessage = (e: WebSocketMessageEvent) => {
      try {
        const data = JSON.parse(e.data);
        const type = data.type || 'message';
        this.emit(type, data);
      } catch {
        // ignore non-JSON messages
      }
    };

    this.ws.onclose = () => {
      this.emit('disconnected', {});
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      // onclose will handle reconnect
    };
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;
    // Exponential backoff: 3s → 6s → 12s → 24s → 30s cap
    const delay = Math.min(
      3000 * Math.pow(2, this.reconnectAttempts),
      WsClient.MAX_RECONNECT_MS,
    );
    this.reconnectAttempts++;
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.doConnect();
    }, delay);
  }

  send(msg: Record<string, unknown>): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  on(type: string, listener: WsListener): () => void {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(listener);
    return () => {
      this.listeners.get(type)?.delete(listener);
    };
  }

  private emit(type: string, data: any): void {
    this.listeners.get(type)?.forEach((fn) => fn(data));
  }

  disconnect(): void {
    // ── js-memory-leaks: Clear timer and listeners ──
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.listeners.clear();
  }
}

export const wsService = new WsClient();
