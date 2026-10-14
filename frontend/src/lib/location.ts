import type { FieldErrors } from "./geofenceValidation";

/** The backend rejects timestamps more than 5 minutes ahead of its clock. */
export const MAX_FUTURE_SKEW_MS = 5 * 60 * 1000;

export interface LocationFormValues {
  vehicleId: string;
  latitude: string;
  longitude: string;
  /** Value of a datetime-local input (local time, no zone). */
  timestamp: string;
}

function pad(n: number) {
  return String(n).padStart(2, "0");
}

/** Formats a Date for <input type="datetime-local" step="1"> in local time. */
export function toDateTimeLocal(d: Date): string {
  return (
    `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}` +
    `T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
  );
}

/** Converts a datetime-local value (local time) to RFC 3339 UTC without millis. */
export function localToIsoUtc(value: string): string | null {
  const d = new Date(value);
  if (!value || Number.isNaN(d.getTime())) return null;
  return d.toISOString().replace(/\.\d{3}Z$/, "Z");
}

function parseCoordinate(raw: string): number | null {
  if (raw.trim() === "") return null;
  const n = Number(raw);
  return Number.isFinite(n) ? n : NaN;
}

/** Mirrors backend ValidateLocationUpdate. Keys match its `details`. */
export function validateLocation(values: LocationFormValues, now = new Date()): FieldErrors {
  const errors: FieldErrors = {};
  if (!values.vehicleId) errors.vehicle_id = "is required";

  const lat = parseCoordinate(values.latitude);
  if (lat === null) errors.latitude = "is required";
  else if (Number.isNaN(lat) || lat < -90 || lat > 90)
    errors.latitude = "must be between -90 and 90";

  const lng = parseCoordinate(values.longitude);
  if (lng === null) errors.longitude = "is required";
  else if (Number.isNaN(lng) || lng < -180 || lng > 180) {
    errors.longitude = "must be between -180 and 180";
  }

  const iso = localToIsoUtc(values.timestamp);
  if (!values.timestamp) errors.timestamp = "is required";
  else if (!iso) errors.timestamp = "must be a valid date and time";
  else if (new Date(iso).getTime() > now.getTime() + MAX_FUTURE_SKEW_MS) {
    errors.timestamp = "must not be more than 5 minutes in the future";
  }
  return errors;
}

/** Geofence ids entered and left between two membership sets. */
export function diffGeofences(
  prev: string[],
  next: string[],
): { entered: string[]; left: string[] } {
  const before = new Set(prev);
  const after = new Set(next);
  return {
    entered: next.filter((id) => !before.has(id)),
    left: prev.filter((id) => !after.has(id)),
  };
}
