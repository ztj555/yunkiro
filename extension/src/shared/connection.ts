export type ConnectionState = 'disconnected' | 'connecting' | 'connected';

export interface ConnectionEvents {
  onOpen: () => void;
  onClose: () => void;
  onMessage: (data: string) => void;
  onError: (error: Event) => void;
}

/**
 * WebSocket wrapper with auto-reconnect and exponential backoff.
 * Designed for use in the extension's service worker (background script).
 */
export class WebSocketConnection {
  private ws: WebSocket | null = null;
  private url: string;
  private events: ConnectionEvents;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private reconnectDelay = 1000;
  private readonly maxReconnectDelay = 10000;
  private shouldReconnect = true;
  private _state: ConnectionState = 'disconnected';
  private messageQueue: string[] = [];

  constructor(url: string, events: ConnectionEvents) {
    this.url = url;
    this.events = events;
  }

  get state(): ConnectionState {
    return this._state;
  }

  connect(): void {
    if (this._state === 'connecting' || this._state === 'connected') {
      return;
    }

    this.shouldReconnect = true;
    this._state = 'connecting';

    try {
      this.ws = new WebSocket(this.url);
    } catch {
      this._state = 'disconnected';
      this.scheduleReconnect();
      return;
    }

    this.ws.onopen = () => {
      this._state = 'connected';
      this.reconnectDelay = 1000;
      this.flushQueue();
      this.events.onOpen();
    };

    this.ws.onclose = () => {
      this._state = 'disconnected';
      this.ws = null;
      this.events.onClose();
      this.scheduleReconnect();
    };

    this.ws.onerror = (event) => {
      this.events.onError(event);
    };

    this.ws.onmessage = (event) => {
      this.events.onMessage(event.data as string);
    };
  }

  disconnect(): void {
    this.shouldReconnect = false;
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
      this.reconnectTimeout = null;
    }
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this._state = 'disconnected';
  }

  send(data: string): void {
    if (this._state === 'connected' && this.ws) {
      this.ws.send(data);
    } else {
      this.messageQueue.push(data);
    }
  }

  private flushQueue(): void {
    while (this.messageQueue.length > 0 && this._state === 'connected' && this.ws) {
      const msg = this.messageQueue.shift()!;
      this.ws.send(msg);
    }
  }

  private scheduleReconnect(): void {
    if (!this.shouldReconnect) return;

    this.reconnectTimeout = setTimeout(() => {
      this.reconnectTimeout = null;
      this.connect();
    }, this.reconnectDelay);

    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
  }
}
