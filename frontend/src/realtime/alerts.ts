import type { AlertMessage } from "../api/types";

/** Feed size cap. */
export const MAX_ALERTS = 100;

/**
 * Merges alert lists (each newest first), keeping the first occurrence of
 * every event_id, capped at MAX_ALERTS. Earlier lists win, so live alerts
 * keep their place when the same alert also arrives via history.
 */
export function mergeAlerts(...lists: AlertMessage[][]): AlertMessage[] {
  const seen = new Set<string>();
  const out: AlertMessage[] = [];
  for (const list of lists) {
    for (const a of list) {
      if (seen.has(a.event_id)) continue;
      seen.add(a.event_id);
      out.push(a);
      if (out.length === MAX_ALERTS) return out;
    }
  }
  return out;
}

/** Restricted-zone entries are the alerts that matter most. */
export function isCritical(a: AlertMessage): boolean {
  return a.geofence.category === "restricted_zone" && a.event_type === "entry";
}
