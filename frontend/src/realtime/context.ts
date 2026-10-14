import { createContext, useContext } from "react";

import type { AlertMessage, LatLng } from "../api/types";
import type { SocketStatus } from "./alertSocket";

/** A short-lived highlight on the map where an alert just happened. */
export interface AlertPulse {
  id: string;
  point: LatLng;
  critical: boolean;
}

export interface AlertSocketValue {
  status: SocketStatus;
  /** Newest first: live alerts merged with history, deduped, capped at 100. */
  alerts: AlertMessage[];
  /** True until the history seed has loaded. */
  loadingHistory: boolean;
  /** Alerts received live this session (for "unseen" badges). */
  liveCount: number;
  pulses: AlertPulse[];
}

export const AlertSocketContext = createContext<AlertSocketValue | null>(null);

export function useAlertSocket(): AlertSocketValue {
  const value = useContext(AlertSocketContext);
  if (!value) throw new Error("useAlertSocket must be used inside <AlertSocketProvider>");
  return value;
}

/** For components that may render outside the provider (e.g. in isolation). */
export function useOptionalAlertSocket(): AlertSocketValue | null {
  return useContext(AlertSocketContext);
}
