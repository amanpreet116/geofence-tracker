// Framework-free WebSocket client for /ws/alerts: reconnects with
// exponential backoff + full jitter, retries immediately on window focus /
// network online, and detects dead connections with an app-level ping.

import type { WsMessage } from "../api/types";

export type SocketStatus = "connecting" | "open" | "reconnecting" | "closed";

export interface AlertSocketOptions {
  url: string;
  onStatus: (status: SocketStatus) => void;
  onMessage: (message: WsMessage) => void;
  /** Injectable for tests. */
  WebSocketImpl?: typeof WebSocket;
  random?: () => number;
  baseDelayMs?: number;
  maxDelayMs?: number;
  /** Consecutive failures after which status reports "closed" (still retrying). */
  offlineAfterFailures?: number;
  pingIntervalMs?: number;
  /** No message for this long after a ping → assume the connection is dead. */
  pongTimeoutMs?: number;
}

/**
 * Delay before reconnect attempt `attempt` (0-based): "full jitter",
 * uniformly random in [0, min(max, base·2^attempt)]. Jitter spreads out
 * clients that all lost the server at the same moment.
 */
export function backoffDelay(
  attempt: number,
  { baseMs = 500, maxMs = 30_000, random = Math.random } = {},
): number {
  const ceiling = Math.min(maxMs, baseMs * 2 ** attempt);
  return Math.round(random() * ceiling);
}

export class AlertSocket {
  private readonly opts: Required<Omit<AlertSocketOptions, "WebSocketImpl">> & {
    WebSocketImpl: typeof WebSocket;
  };
  private ws: WebSocket | null = null;
  private status: SocketStatus | null = null;
  private failures = 0;
  private stopped = true;
  private connectTimer: ReturnType<typeof setTimeout> | undefined;
  private pingTimer: ReturnType<typeof setInterval> | undefined;
  private pongTimer: ReturnType<typeof setTimeout> | undefined;

  constructor(options: AlertSocketOptions) {
    this.opts = {
      WebSocketImpl: options.WebSocketImpl ?? WebSocket,
      random: Math.random,
      baseDelayMs: 500,
      maxDelayMs: 30_000,
      offlineAfterFailures: 3,
      pingIntervalMs: 25_000,
      pongTimeoutMs: 10_000,
      ...options,
    };
  }

  /**
   * Connects on the next tick. Deferring makes React StrictMode's
   * mount → unmount → mount open a single socket: the first start() is
   * cancelled by stop() before it ever connects.
   */
  start(): void {
    if (!this.stopped) return;
    this.stopped = false;
    this.setStatus("connecting");
    this.connectTimer = setTimeout(() => this.connect(), 0);
    if (typeof window !== "undefined") {
      window.addEventListener("focus", this.retryNow);
      window.addEventListener("online", this.retryNow);
      window.addEventListener("offline", this.onOffline);
    }
  }

  stop(): void {
    this.stopped = true;
    clearTimeout(this.connectTimer);
    this.stopHeartbeat();
    if (typeof window !== "undefined") {
      window.removeEventListener("focus", this.retryNow);
      window.removeEventListener("online", this.retryNow);
      window.removeEventListener("offline", this.onOffline);
    }
    const ws = this.ws;
    this.ws = null;
    if (ws) {
      ws.onopen = ws.onmessage = ws.onclose = ws.onerror = null;
      ws.close(1000, "client stopped");
    }
  }

  private setStatus(s: SocketStatus) {
    if (s === this.status) return;
    this.status = s;
    this.opts.onStatus(s);
  }

  private isOffline() {
    return typeof navigator !== "undefined" && navigator.onLine === false;
  }

  private connect() {
    if (this.stopped) return;
    let ws: WebSocket;
    try {
      ws = new this.opts.WebSocketImpl(this.opts.url);
    } catch {
      this.scheduleReconnect();
      return;
    }
    this.ws = ws;

    ws.onopen = () => {
      this.failures = 0;
      this.setStatus("open");
      this.startHeartbeat();
    };
    ws.onmessage = (event: MessageEvent) => {
      this.markAlive();
      let msg: WsMessage;
      try {
        msg = JSON.parse(String(event.data)) as WsMessage;
      } catch {
        return; // not JSON: ignore
      }
      this.opts.onMessage(msg);
    };
    // onerror is always followed by onclose; reconnect from there.
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.stopHeartbeat();
      this.scheduleReconnect();
    };
  }

  private scheduleReconnect() {
    if (this.stopped) return;
    const delay = backoffDelay(this.failures, {
      baseMs: this.opts.baseDelayMs,
      maxMs: this.opts.maxDelayMs,
      random: this.opts.random,
    });
    this.failures += 1;
    this.setStatus(
      this.isOffline() || this.failures >= this.opts.offlineAfterFailures
        ? "closed"
        : "reconnecting",
    );
    clearTimeout(this.connectTimer);
    this.connectTimer = setTimeout(() => this.connect(), delay);
  }

  /** Window regained focus or network came back: skip the remaining backoff. */
  private readonly retryNow = () => {
    if (this.stopped || this.ws) return; // connected or a connect is in flight
    clearTimeout(this.connectTimer);
    this.failures = 0;
    this.connect();
  };

  private readonly onOffline = () => {
    if (this.status !== "open") this.setStatus("closed");
  };

  private startHeartbeat() {
    this.stopHeartbeat();
    this.pingTimer = setInterval(() => {
      const ws = this.ws;
      if (!ws || ws.readyState !== ws.OPEN) return;
      ws.send(JSON.stringify({ type: "ping" }));
      clearTimeout(this.pongTimer);
      this.pongTimer = setTimeout(() => ws.close(4000, "pong timeout"), this.opts.pongTimeoutMs);
    }, this.opts.pingIntervalMs);
  }

  /** Any inbound message proves the connection is alive. */
  private markAlive() {
    clearTimeout(this.pongTimer);
  }

  private stopHeartbeat() {
    clearInterval(this.pingTimer);
    clearTimeout(this.pongTimer);
  }
}
