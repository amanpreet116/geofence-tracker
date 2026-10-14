import type { LatLng } from "../api/types";

/** Rounds to 6 decimals (~0.1 m), enough for geofences and readable in text. */
export function round6(n: number): number {
  return Math.round(n * 1e6) / 1e6;
}

export function samePoint(a: LatLng | undefined, b: LatLng | undefined): boolean {
  return a !== undefined && b !== undefined && a[0] === b[0] && a[1] === b[1];
}

/** A ring is closed when it has ≥ 4 points and the last repeats the first. */
export function isClosedRing(points: LatLng[]): boolean {
  return points.length >= 4 && samePoint(points[0], points[points.length - 1]);
}

export function uniquePointCount(points: LatLng[]): number {
  return new Set(points.map(([lat, lng]) => `${lat},${lng}`)).size;
}

/** Appends the first point unless the ring is already closed. */
export function closeRing(points: LatLng[]): LatLng[] {
  const first = points[0];
  if (!first || samePoint(first, points[points.length - 1])) return points;
  return [...points, first];
}

/** Formats points as pretty JSON, one [lat, lng] pair per line. */
export function formatCoordinates(points: LatLng[]): string {
  if (points.length === 0) return "";
  return "[\n" + points.map(([lat, lng]) => `  [${lat}, ${lng}]`).join(",\n") + "\n]";
}

export type ParseResult = { ok: true; points: LatLng[] } | { ok: false; error: string };

/**
 * Parses either JSON ([[lat, lng], ...]) or one "lat,lng" (or "lat lng")
 * pair per line. Values are [latitude, longitude], like the API. Range and
 * polygon rules are checked separately by validateGeofence.
 */
export function parseCoordinates(text: string): ParseResult {
  const trimmed = text.trim();
  if (trimmed === "") return { ok: false, error: "Enter at least 4 coordinate pairs." };
  return /^[[{]/.test(trimmed) ? parseJson(trimmed) : parseLines(trimmed);
}

function parseJson(text: string): ParseResult {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    return {
      ok: false,
      error: "Invalid JSON. Expected [[lat, lng], …], e.g. [[37.77, -122.41], …].",
    };
  }
  if (!Array.isArray(data)) return { ok: false, error: "Expected an array of [lat, lng] pairs." };
  const points: LatLng[] = [];
  for (const [i, pair] of (data as unknown[]).entries()) {
    if (
      !Array.isArray(pair) ||
      pair.length !== 2 ||
      !pair.every((v) => typeof v === "number" && Number.isFinite(v))
    ) {
      return { ok: false, error: `Item ${i + 1} must be a [latitude, longitude] pair of numbers.` };
    }
    points.push([pair[0] as number, pair[1] as number]);
  }
  return { ok: true, points };
}

function parseLines(text: string): ParseResult {
  const points: LatLng[] = [];
  for (const [i, raw] of text.split(/\r?\n/).entries()) {
    const line = raw.trim();
    if (line === "") continue;
    const parts = line.split(/[\s,]+/).filter(Boolean);
    const nums = parts.map(Number);
    if (parts.length !== 2 || !nums.every(Number.isFinite)) {
      return {
        ok: false,
        error: `Line ${i + 1}: expected "latitude, longitude" (e.g. 37.7749, -122.4194), got "${line}".`,
      };
    }
    points.push([nums[0] as number, nums[1] as number]);
  }
  return { ok: true, points };
}
