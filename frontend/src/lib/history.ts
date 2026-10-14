import type { Violation, ViolationFilters } from "../api/types";

export const PAGE_SIZES = [25, 50, 100, 500] as const;
export type PageSize = (typeof PAGE_SIZES)[number];
const DEFAULT_PAGE_SIZE: PageSize = 50;

/** History view state, as stored in the URL (?vehicle_id=&geofence_id=&start=&end=&limit=&page=). */
export interface HistoryState {
  vehicleId: string;
  geofenceId: string;
  /** RFC 3339 UTC ("" = unbounded); the URL form, so a shared link means the same instant. */
  startDate: string;
  endDate: string;
  limit: PageSize;
  /** 1-based. */
  page: number;
}

/** Reads state from search params, ignoring anything malformed. */
export function readHistoryState(params: URLSearchParams): HistoryState {
  const limit = Number(params.get("limit"));
  const page = Number(params.get("page"));
  const iso = (v: string | null) => {
    const d = v ? new Date(v) : null;
    return d && !Number.isNaN(d.getTime()) ? d.toISOString().replace(/\.\d{3}Z$/, "Z") : "";
  };
  return {
    vehicleId: params.get("vehicle_id") ?? "",
    geofenceId: params.get("geofence_id") ?? "",
    startDate: iso(params.get("start_date")),
    endDate: iso(params.get("end_date")),
    limit: (PAGE_SIZES as readonly number[]).includes(limit)
      ? (limit as PageSize)
      : DEFAULT_PAGE_SIZE,
    page: Number.isInteger(page) && page >= 1 ? page : 1,
  };
}

/** Serializes state to search params, omitting defaults. */
export function writeHistoryState(s: HistoryState): URLSearchParams {
  const p = new URLSearchParams();
  if (s.vehicleId) p.set("vehicle_id", s.vehicleId);
  if (s.geofenceId) p.set("geofence_id", s.geofenceId);
  if (s.startDate) p.set("start_date", s.startDate);
  if (s.endDate) p.set("end_date", s.endDate);
  if (s.limit !== DEFAULT_PAGE_SIZE) p.set("limit", String(s.limit));
  if (s.page > 1) p.set("page", String(s.page));
  return p;
}

/** API filters for the state: page → offset. */
export function toViolationFilters(s: HistoryState): ViolationFilters {
  return {
    vehicle_id: s.vehicleId || undefined,
    geofence_id: s.geofenceId || undefined,
    start_date: s.startDate || undefined,
    end_date: s.endDate || undefined,
    limit: s.limit,
    offset: (s.page - 1) * s.limit,
  };
}

/** "Showing 51–100 of 245" numbers; from/to are 0 when there is nothing. */
export function pageRange(page: number, limit: number, total: number) {
  const pages = Math.max(1, Math.ceil(total / limit));
  const from = total === 0 ? 0 : Math.min((page - 1) * limit + 1, total);
  const to = Math.min(page * limit, total);
  return { pages, from, to };
}

/**
 * Page numbers to show around `current`, with null for gaps:
 * [1, null, 4, 5, 6, null, 20].
 */
export function pageList(current: number, pages: number, siblings = 1): (number | null)[] {
  const wanted = new Set([1, pages]);
  for (let p = current - siblings; p <= current + siblings; p++) {
    if (p >= 1 && p <= pages) wanted.add(p);
  }
  const sorted = [...wanted].sort((a, b) => a - b);
  const out: (number | null)[] = [];
  sorted.forEach((p, i) => {
    const prev = sorted[i - 1];
    if (prev !== undefined && p - prev === 2) {
      out.push(prev + 1); // a one-page gap: show the page instead of "…"
    } else if (prev !== undefined && p - prev > 2) {
      out.push(null);
    }
    out.push(p);
  });
  return out;
}

function csvCell(value: string | number): string {
  const s = String(value);
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
}

/** RFC 4180 CSV of violation rows (API field names as headers). */
export function violationsToCsv(rows: Violation[]): string {
  const headers = [
    "id",
    "timestamp",
    "vehicle_id",
    "vehicle_number",
    "geofence_id",
    "geofence_name",
    "event_type",
    "latitude",
    "longitude",
  ] as const;
  const lines = [headers.join(",")];
  for (const r of rows) lines.push(headers.map((h) => csvCell(r[h])).join(","));
  return lines.join("\r\n") + "\r\n";
}
