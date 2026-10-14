import { GEOFENCE_CATEGORIES, type GeofenceCategory, type LatLng } from "../api/types";
import { isClosedRing, samePoint, uniquePointCount } from "./coordinates";

export interface GeofenceFormValues {
  name: string;
  description: string;
  category: GeofenceCategory | "";
  coordinates: LatLng[];
}

/**
 * Field errors keyed exactly like the backend's validation `details`
 * ("name", "category", "coordinates", "coordinates[i]"), so client and
 * server errors render in the same places.
 */
export type FieldErrors = Record<string, string>;

const MAX_NAME_LENGTH = 200;
export const MAX_POINTS = 1000;

/** An edge spanning > 180° between interior longitudes, e.g. 179 → -179 (planar geometry can't wrap). */
function crossesAntimeridian(points: LatLng[]): boolean {
  return points.some((p, i) => {
    const prev = points[i - 1];
    if (!prev) return false;
    const [a, b] = [prev[1], p[1]];
    return Math.abs(b - a) > 180 && Math.abs(a) !== 180 && Math.abs(b) !== 180;
  });
}

/** Mirrors backend service.ValidateCreateGeofence (messages included). */
export function validateGeofence(values: GeofenceFormValues): FieldErrors {
  const errors: FieldErrors = {};
  const add = (field: string, msg: string) => {
    errors[field] = errors[field] ? `${errors[field]}; ${msg}` : msg;
  };

  const name = values.name.trim();
  if (!name) add("name", "is required");
  else if ([...name].length > MAX_NAME_LENGTH)
    add("name", `must be at most ${MAX_NAME_LENGTH} characters`);

  if (!values.category) add("category", "is required");
  else if (!GEOFENCE_CATEGORIES.includes(values.category)) {
    add("category", `must be one of ${GEOFENCE_CATEGORIES.join(", ")}`);
  }

  const points = values.coordinates;
  if (points.length === 0) {
    add("coordinates", "is required");
    return errors;
  }
  if (points.length > MAX_POINTS) {
    add("coordinates", `must contain at most ${MAX_POINTS} points`);
    return errors;
  }

  let pairsOk = true;
  points.forEach(([lat, lng], i) => {
    if (lat < -90 || lat > 90) {
      add(`coordinates[${i}]`, "latitude must be between -90 and 90");
      pairsOk = false;
    }
    if (lng < -180 || lng > 180) {
      add(`coordinates[${i}]`, "longitude must be between -180 and 180");
      pairsOk = false;
    }
  });

  if (points.length < 4) {
    add("coordinates", "must contain at least 4 points (3 unique points plus the closing point)");
    return errors;
  }
  if (!pairsOk) return errors;
  if (!samePoint(points[0], points[points.length - 1])) {
    add("coordinates", "polygon must be closed: first and last points must be identical");
  }
  if (uniquePointCount(points) < 3) add("coordinates", "must contain at least 3 unique points");
  if (crossesAntimeridian(points)) {
    add(
      "coordinates",
      "polygon must not cross the antimeridian (±180° longitude); split it into two geofences",
    );
  }
  return errors;
}

/** Errors for "coordinates[i]" keys as readable lines, in point order. */
export function pointErrors(errors: FieldErrors): string[] {
  return Object.entries(errors)
    .map(([key, msg]) => ({ match: /^coordinates\[(\d+)\]$/.exec(key), msg }))
    .filter((e): e is { match: RegExpExecArray; msg: string } => e.match !== null)
    .sort((a, b) => Number(a.match[1]) - Number(b.match[1]))
    .map(({ match, msg }) => `Point ${Number(match[1]) + 1}: ${msg}`);
}

export { isClosedRing };
