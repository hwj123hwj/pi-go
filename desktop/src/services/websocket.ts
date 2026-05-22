// websocket.ts — WebSocket connection management with auto-reconnect.

type MessageHandler = (data: any) => void;

class WebSocketService {
  private ws: WebSocket | null = null;
  private url: string = '';
  private handlers: Map<string, MessageHandler[]> = new Map();
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 20;
  private baseDelay = 1000;
  private maxDelay = 30000;
  private _connected = false;

  get connected(): boolean {
    return this._connected;
  }

  connect(url: string): void {
    // If already connected to the same URL, skip
    if (this._connected && this.ws && this.ws.readyState === WebSocket.OPEN) {
      return;
    }

    // Clean up any existing connection/reconnect
    this.disconnect();

    // Strip http(s) and use ws(s)
    const wsUrl = url.replace(/^http/, 'ws');
    this.url = `${wsUrl}/ws`;

    console.log(`[ws] Connecting to ${this.url}`);
    this.createConnection();
  }

  private createConnection(): void {
    if (this.ws) {
      // Remove listeners to prevent triggering reconnect on intentional close
      this.ws.onclose = null;
      this.ws.onerror = null;
      this.ws.close();
    }

    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('[ws] Connected');
      this._connected = true;
      this.reconnectAttempts = 0;
      this.emit('connected', {});
    };

    this.ws.onclose = () => {
      console.log('[ws] Disconnected');
      this._connected = false;
      this.emit('disconnected', {});
      this.scheduleReconnect();
    };

    this.ws.onerror = (err) => {
      console.error('[ws] Error', err);
      this.emit('error', { error: 'WebSocket error' });
    };

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        this.routeMessage(data);
      } catch (err) {
        console.error('[ws] Failed to parse message', err);
      }
    };
  }

  private routeMessage(data: any): void {
    // Server messages have the form: { type, session_id, event, ... }
    const type = data.type;

    // Emit general message
    this.emit('message', data);

    // Emit type-specific messages
    if (type === 'event' && data.event) {
      this.emit(`event:${data.event.type}`, data);
    } else {
      this.emit(`type:${type}`, data);
    }
  }

  send(data: object): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    } else {
      console.warn('[ws] Cannot send, not connected');
    }
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this._connected = false;
  }

  on(event: string, handler: MessageHandler): () => void {
    if (!this.handlers.has(event)) {
      this.handlers.set(event, []);
    }
    this.handlers.get(event)!.push(handler);

    // Return unsubscribe function
    return () => {
      const handlers = this.handlers.get(event);
      if (handlers) {
        const idx = handlers.indexOf(handler);
        if (idx >= 0) handlers.splice(idx, 1);
      }
    };
  }

  private emit(event: string, data: any): void {
    const handlers = this.handlers.get(event);
    if (handlers) {
      handlers.forEach((h) => h(data));
    }
  }

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('[ws] Max reconnect attempts reached');
      return;
    }

    const delay = Math.min(
      this.baseDelay * Math.pow(2, this.reconnectAttempts),
      this.maxDelay
    );
    this.reconnectAttempts++;

    console.log(`[ws] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);

    this.reconnectTimer = setTimeout(() => {
      this.createConnection();
    }, delay);
  }
}

// Singleton
export const wsService = new WebSocketService();
