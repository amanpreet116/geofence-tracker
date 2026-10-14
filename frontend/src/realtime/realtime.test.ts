import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { AlertMessage } from "../api/types";
import { formatRelative } from "../lib/format";
import { AlertSocket, backoffDelay, type SocketStatus } from "./alertSocket";
import { MAX_ALERTS, mergeAlerts } from "./alerts";

/** Minimal controllable WebSocket double. */
class FakeSocket {
  static instances: FakeSocket[] = [];
  readonly OPEN = 1;
  readyState = 0;
  sent: string[] = [];
  closed = false;
  onopen: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  constructor(public url: string) {
    FakeSocket.instances.push(this);
  }
  send(data: string) {
    this.sent.push(data);
  }
  close() {
    if (this.closed) return;
    this.closed = true;
    this.readyState = 3;
    this.onclose?.();
  }
  // test helpers
  open() {
    this.readyState = 1;
    this.onopen?.();
  }
  receive(msg: unknown) {
    this.onmessage?.({ data: JSON.stringify(msg) });
  }
  drop() {
    this.close();
  }
}

const alert = (id: string): AlertMessage => ({
  event_id: id,
  event_type: "entry",
  timestamp: "2025-01-15T10:35:00Z",
  vehicle: { vehicle_id: "veh_1", vehicle_number: "KA-01", driver_name: "John" },
  geofence: { geofence_id: "geo_1", geofence_name: "Zone", category: "restricted_zone" },
  location: { latitude: 1, longitude: 2 },
});

describe("backoffDelay", () => {
  it("grows exponentially and caps at maxMs", () => {
    const top = { random: () => 1 };
    expect([0, 1, 2, 3, 4].map((a) => backoffDelay(a, top))).toEqual([500, 1000, 2000, 4000, 8000]);
    expect(backoffDelay(20, top)).toBe(30_000);
  });
  it("applies full jitter", () => {
    expect(backoffDelay(3, { random: () => 0 })).toBe(0);
    expect(backoffDelay(3, { random: () => 0.5 })).toBe(2000);
  });
});

describe("mergeAlerts", () => {
  it("dedupes by event_id keeping the first list's order, capped", () => {
    expect(
      mergeAlerts([alert("b"), alert("a")], [alert("a"), alert("c")]).map((a) => a.event_id),
    ).toEqual(["b", "a", "c"]);
    const many = Array.from({ length: 150 }, (_, i) => alert(`e${i}`));
    expect(mergeAlerts(many)).toHaveLength(MAX_ALERTS);
  });
});

describe("formatRelative", () => {
  const now = new Date("2025-01-15T10:35:00Z").getTime();
  it.each([
    ["2025-01-15T10:34:58Z", "just now"],
    ["2025-01-15T10:34:15Z", "45s ago"],
    ["2025-01-15T10:30:00Z", "5m ago"],
    ["2025-01-15T07:35:00Z", "3h ago"],
    ["2025-01-12T10:35:00Z", "3d ago"],
  ])("%s → %s", (iso, want) => expect(formatRelative(iso, now)).toBe(want));
});

describe("AlertSocket", () => {
  let statuses: SocketStatus[];
  let messages: unknown[];
  const make = () =>
    new AlertSocket({
      url: "ws://test/ws/alerts",
      onStatus: (s) => statuses.push(s),
      onMessage: (m) => messages.push(m),
      WebSocketImpl: FakeSocket as unknown as typeof WebSocket,
      random: () => 1, // deterministic: delay = ceiling
    });

  beforeEach(() => {
    vi.useFakeTimers();
    // Vitest runs in Node: give the socket a window to attach focus/online listeners to.
    vi.stubGlobal("window", new EventTarget());
    FakeSocket.instances = [];
    statuses = [];
    messages = [];
  });
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("opens exactly one socket under StrictMode-style start/stop/start", () => {
    const s = make();
    s.start();
    s.stop(); // StrictMode's simulated unmount before the deferred connect
    s.start();
    vi.advanceTimersByTime(0);
    expect(FakeSocket.instances).toHaveLength(1);
    s.stop();
  });

  it("reports status, delivers messages and ignores non-JSON", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    const ws = FakeSocket.instances[0]!;
    ws.open();
    ws.receive({ type: "connected" });
    ws.onmessage?.({ data: "not json" });
    expect(statuses).toEqual(["connecting", "open"]);
    expect(messages).toEqual([{ type: "connected" }]);
    s.stop();
  });

  it("reconnects with exponential backoff, then reports closed while still retrying", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    FakeSocket.instances[0]!.open();

    FakeSocket.instances[0]!.drop(); // failure 1 → wait 500
    expect(statuses.at(-1)).toBe("reconnecting");
    vi.advanceTimersByTime(499);
    expect(FakeSocket.instances).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(FakeSocket.instances).toHaveLength(2);

    FakeSocket.instances[1]!.drop(); // failure 2 → wait 1000
    vi.advanceTimersByTime(1000);
    FakeSocket.instances[2]!.drop(); // failure 3 → "closed", wait 2000
    expect(statuses.at(-1)).toBe("closed");
    vi.advanceTimersByTime(2000);
    expect(FakeSocket.instances).toHaveLength(4);

    FakeSocket.instances[3]!.open(); // success resets the backoff
    expect(statuses.at(-1)).toBe("open");
    FakeSocket.instances[3]!.drop();
    vi.advanceTimersByTime(500);
    expect(FakeSocket.instances).toHaveLength(5);
    s.stop();
  });

  it("retries immediately on window focus/online instead of waiting", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    FakeSocket.instances[0]!.drop();
    FakeSocket.instances.length = 0;
    window.dispatchEvent(new Event("online"));
    expect(FakeSocket.instances).toHaveLength(1);
    window.dispatchEvent(new Event("focus")); // a connect is in flight: no duplicate
    expect(FakeSocket.instances).toHaveLength(1);
    s.stop();
  });

  it("closes a silent connection after a missed pong", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    const ws = FakeSocket.instances[0]!;
    ws.open();
    vi.advanceTimersByTime(25_000);
    expect(ws.sent).toEqual(['{"type":"ping"}']);
    vi.advanceTimersByTime(10_000);
    expect(ws.closed).toBe(true);
    expect(statuses.at(-1)).toBe("reconnecting");
    s.stop();
  });

  it("a reply keeps the connection open", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    const ws = FakeSocket.instances[0]!;
    ws.open();
    vi.advanceTimersByTime(25_000);
    ws.receive({ type: "pong" });
    vi.advanceTimersByTime(10_000);
    expect(ws.closed).toBe(false);
    s.stop();
  });

  it("stop() closes the socket and never reconnects", () => {
    const s = make();
    s.start();
    vi.advanceTimersByTime(0);
    FakeSocket.instances[0]!.open();
    s.stop();
    vi.advanceTimersByTime(60_000);
    expect(FakeSocket.instances).toHaveLength(1);
    expect(FakeSocket.instances[0]!.closed).toBe(true);
  });
});
