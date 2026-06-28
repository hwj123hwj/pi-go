/**
 * ws.ts — Lightweight WebSocket client for Pi-Go mobile
 *
 * The pi-go backend uses WebSocket for streaming chat events:
 * Client sends: { type: "prompt", session_id, prompt }
 * Server sends: { type: "event:text_delta", session_id, event: { text_delta } }
 *               { type: "event:tool_start", ... }
 *               { type: "event:tool_end", ... }
 *               { type: "status", session_id, streaming }
 */

type WsListener = (data: any) => void;

class WsClient {
  private ws: WebSocket | null = null;
  private listeners = new Map<string, Set<WsListener>>();
  private url = '';
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  connect(url: string): void {
    this.url = url.replace(/^http/, 'ws');
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
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      this.doConnect();
    }, 3000);
  }

  send(msg: Record<string, unknown>): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  on(type: string, listener: WsListener): () => void {
    if (!this.listeners.has(type)) this.listeners.set(type, new Set());
    this.listeners.get(type)!.add(listener);
    return () => this.listeners.get(type)?.delete(listener);
  }

  private emit(type: string, data: any): void {
    this.listeners.get(type)?.forEach((fn) => fn(data));
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
  }
}

export const wsService = new WsClient();
