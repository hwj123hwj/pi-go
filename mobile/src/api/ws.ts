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
  // ── BUGFIX 12/13: Track lifecycle to prevent zombie reconnects ──
  private connected = false; // connect() was called (not disconnect()ed)
  private wsId = 0;          // Incremented on each new WebSocket; stale callbacks check this

  connect(url: string): void {
    const newUrl = url.replace(/^http/, 'ws') + '/ws';

    // ── BUGFIX 12: Close previous connection if switching URLs ──
    if (this.ws && newUrl !== this.url) {
      this.doDisconnect();
    }

    this.url = newUrl;
    this.connected = true;
    this.reconnectAttempts = 0;
    this.doConnect();
  }

  private doConnect(): void {
    if (!this.connected) return; // Don't reconnect after explicit disconnect

    const myWsId = ++this.wsId; // Unique ID for this WebSocket instance
    let ws: WebSocket;
    try {
      ws = new WebSocket(this.url);
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;

    ws.onopen = () => {
      if (myWsId !== this.wsId) return; // Stale callback
      this.reconnectAttempts = 0;
      this.emit('connected', {});
    };

    ws.onmessage = (e: WebSocketMessageEvent) => {
      if (myWsId !== this.wsId) return; // Stale callback
      try {
        const data = JSON.parse(e.data);
        const type = data.type || 'message';
        this.emit(type, data);
      } catch {
        // ignore non-JSON messages
      }
    };

    ws.onclose = () => {
      if (myWsId !== this.wsId) return; // Stale callback — don't reconnect
      this.emit('disconnected', {});
      this.scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose will handle reconnect
    };
  }

  // ── BUGFIX 12: Internal disconnect without clearing listeners ──
  private doDisconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      const old = this.ws;
      old.onopen = null;
      old.onmessage = null;
      old.onclose = null;
      old.onerror = null;
      try { old.close(); } catch { /* ignore */ }
      this.ws = null;
    }
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
    // ── BUGFIX 13: Set connected=false BEFORE closing to prevent onclose → scheduleReconnect ──
    this.connected = false;
    this.wsId++;          // Invalidate all stale callbacks
    this.doDisconnect();
    this.listeners.clear();
  }
}

export const wsService = new WsClient();
