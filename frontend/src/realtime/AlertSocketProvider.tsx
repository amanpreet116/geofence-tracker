import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { useAlertNotifications } from "../api/hooks";
import { queryKeys } from "../api/keys";
import { isAlertMessage, type AlertMessage, type WsMessage } from "../api/types";
import { config } from "../config";
import { AlertSocket, type SocketStatus } from "./alertSocket";
import { isCritical, MAX_ALERTS, mergeAlerts } from "./alerts";
import { showAlertToast } from "./showAlertToast";
import { AlertSocketContext, type AlertPulse, type AlertSocketValue } from "./context";

const PULSE_MS = 4000;

/**
 * One WebSocket to /ws/alerts for the whole app. Seeds the feed from
 * GET /alerts/notifications so history survives a reload, then appends live
 * alerts (deduped by event_id). Each live alert toasts, refreshes fleet
 * locations and violation history, and pulses on the map.
 */
export function AlertSocketProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient();
  const history = useAlertNotifications(MAX_ALERTS);
  const [status, setStatus] = useState<SocketStatus>("connecting");
  const [live, setLive] = useState<AlertMessage[]>([]);
  const [pulses, setPulses] = useState<AlertPulse[]>([]);

  // Dedupe outside the state updater: updaters may run lazily or twice
  // (StrictMode), so they can't decide whether to toast.
  const seen = useRef(new Set<string>());

  // Latest handler without re-creating the socket on every render.
  const onAlert = useRef<(a: AlertMessage) => void>(() => {});
  useEffect(() => {
    onAlert.current = (a) => {
      if (seen.current.has(a.event_id)) return;
      seen.current.add(a.event_id);
      setLive((prev) => mergeAlerts([a], prev));
      showAlertToast(a);
      void qc.invalidateQueries({ queryKey: queryKeys.locations.all });
      void qc.invalidateQueries({ queryKey: queryKeys.violations.all });
      const pulse: AlertPulse = {
        id: a.event_id,
        point: [a.location.latitude, a.location.longitude],
        critical: isCritical(a),
      };
      setPulses((p) => [...p.filter((x) => x.id !== pulse.id), pulse]);
      setTimeout(() => setPulses((p) => p.filter((x) => x.id !== pulse.id)), PULSE_MS);
    };
  });

  // After a reconnect, backfill alerts missed while disconnected: re-read
  // history (mergeAlerts dedupes by event_id).
  const wasDisconnected = useRef(false);
  const onStatus = (s: SocketStatus) => {
    setStatus(s);
    if (s === "reconnecting" || s === "closed") wasDisconnected.current = true;
    if (s === "open" && wasDisconnected.current) {
      wasDisconnected.current = false;
      void qc.invalidateQueries({ queryKey: queryKeys.notifications.all });
      void qc.invalidateQueries({ queryKey: queryKeys.locations.all });
    }
  };
  const onStatusRef = useRef(onStatus);
  useEffect(() => {
    onStatusRef.current = onStatus;
  });

  useEffect(() => {
    const socket = new AlertSocket({
      url: config.wsUrl,
      onStatus: (s) => onStatusRef.current(s),
      onMessage: (msg: WsMessage) => {
        if (isAlertMessage(msg)) onAlert.current(msg);
      },
    });
    socket.start();
    return () => socket.stop();
  }, []);

  const value = useMemo<AlertSocketValue>(
    () => ({
      status,
      alerts: mergeAlerts(live, history.data ?? []),
      loadingHistory: history.isPending,
      liveCount: live.length,
      pulses,
    }),
    [status, live, history.data, history.isPending, pulses],
  );

  return <AlertSocketContext value={value}>{children}</AlertSocketContext>;
}
