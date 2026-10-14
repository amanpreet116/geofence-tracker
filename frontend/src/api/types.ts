// Types mirroring the backend JSON exactly (snake_case, as sent).
// Source of truth: docs/ASSIGNMENT.md + docs/EXTRA_ENDPOINTS.md.
// Coordinates are always [latitude, longitude]; timestamps are RFC 3339 UTC.

/** Every response carries the handler execution time as a numeric string. */
export interface Timed {
  time_ns: string;
}

export interface ErrorResponse extends Timed {
  error: string;
  /** Field → message for validation errors; other shapes for some errors (e.g. 409). */
  details?: Record<string, string>;
}

export type LatLng = [latitude: number, longitude: number];

// ---- Geofences ---------------------------------------------------------------

export const GEOFENCE_CATEGORIES = [
  "delivery_zone",
  "restricted_zone",
  "toll_zone",
  "customer_area",
] as const;
export type GeofenceCategory = (typeof GEOFENCE_CATEGORIES)[number];

export interface CreateGeofenceRequest {
  name: string;
  description?: string;
  coordinates: LatLng[];
  category: GeofenceCategory;
}

export interface CreateGeofenceResponse extends Timed {
  id: string;
  name: string;
  status: string;
}

export interface Geofence {
  id: string;
  name: string;
  /** "" when none was given. */
  description: string;
  /** Closed ring: first point repeated as last. */
  coordinates: LatLng[];
  category: GeofenceCategory;
  created_at: string;
}

export interface GeofenceListResponse extends Timed {
  geofences: Geofence[];
}

// ---- Vehicles ----------------------------------------------------------------

export interface CreateVehicleRequest {
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  phone: string;
}

export interface CreateVehicleResponse extends Timed {
  id: string;
  vehicle_number: string;
  status: string;
}

export interface Vehicle {
  id: string;
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  phone: string;
  status: string;
  created_at: string;
}

export interface VehicleListResponse extends Timed {
  vehicles: Vehicle[];
}

// ---- Locations ---------------------------------------------------------------

export interface LocationFix {
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface UpdateLocationRequest {
  vehicle_id: string;
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface UpdateLocationResponse extends Timed {
  vehicle_id: string;
  location_updated: boolean;
  current_geofences: {
    geofence_id: string;
    geofence_name: string;
    status: "inside";
  }[];
}

export interface VehicleLocationResponse extends Timed {
  vehicle_id: string;
  vehicle_number: string;
  /** null until the vehicle's first location update. */
  current_location: LocationFix | null;
  current_geofences: {
    geofence_id: string;
    geofence_name: string;
    category: GeofenceCategory;
  }[];
}

/** GET /fleet/locations (extra endpoint). */
export interface FleetVehicle {
  vehicle_id: string;
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  status: string;
  current_location: LocationFix | null;
  /** Sorted geofence ids; resolve names via GET /geofences. */
  current_geofence_ids: string[];
}

export interface FleetLocationsResponse extends Timed {
  vehicles: FleetVehicle[];
}

// ---- Alert rules -------------------------------------------------------------

export type AlertEventType = "entry" | "exit" | "both";
export type AlertStatus = "active" | "inactive";

export interface ConfigureAlertRequest {
  geofence_id: string;
  /** Omit for a rule that applies to all vehicles. */
  vehicle_id?: string;
  event_type: AlertEventType;
}

export interface ConfigureAlertResponse extends Timed {
  alert_id: string;
  geofence_id: string;
  /** null = all vehicles. */
  vehicle_id: string | null;
  event_type: AlertEventType;
  status: AlertStatus;
}

export interface AlertRule {
  alert_id: string;
  geofence_id: string;
  geofence_name: string;
  /** Both null for rules that apply to all vehicles. */
  vehicle_id: string | null;
  vehicle_number: string | null;
  event_type: AlertEventType;
  status: AlertStatus;
  created_at: string;
}

export interface AlertRuleListResponse extends Timed {
  alerts: AlertRule[];
}

export interface AlertRuleFilters {
  geofence_id?: string;
  vehicle_id?: string;
}

/** PATCH /alerts/{alert_id} (extra endpoint). */
export interface UpdateAlertRuleRequest {
  status: AlertStatus;
}

export type UpdateAlertRuleResponse = AlertRule & Timed;

/** DELETE /alerts/{alert_id} (extra endpoint). */
export interface DeleteAlertRuleResponse extends Timed {
  alert_id: string;
  deleted: true;
}

// ---- Violation history -------------------------------------------------------

export type GeofenceEventType = "entry" | "exit";

export interface Violation {
  id: string;
  vehicle_id: string;
  vehicle_number: string;
  geofence_id: string;
  geofence_name: string;
  event_type: GeofenceEventType;
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface ViolationFilters {
  vehicle_id?: string;
  geofence_id?: string;
  /** RFC 3339. */
  start_date?: string;
  /** RFC 3339. */
  end_date?: string;
  /** Default 50, max 500 (larger values are clamped). */
  limit?: number;
  /** Additive pagination param, default 0. */
  offset?: number;
}

export interface ViolationHistoryResponse extends Timed {
  violations: Violation[];
  /** Matches for the filters, ignoring limit/offset. */
  total_count: number;
}

// ---- Real-time alerts (/ws/alerts) -------------------------------------------

/** The alert message, exactly as specified (no envelope). */
export interface AlertMessage {
  event_id: string;
  event_type: GeofenceEventType;
  timestamp: string;
  vehicle: {
    vehicle_id: string;
    vehicle_number: string;
    driver_name: string;
  };
  geofence: {
    geofence_id: string;
    geofence_name: string;
    category: GeofenceCategory;
  };
  location: {
    latitude: number;
    longitude: number;
  };
}

/** Control messages carry a "type" field; alerts never do. */
export interface ConnectedMessage {
  type: "connected";
  client_id: string;
  server_time: string;
  message: string;
}

export interface PongMessage {
  type: "pong";
}

export type ControlMessage = ConnectedMessage | PongMessage;
export type WsMessage = AlertMessage | ControlMessage;

export function isAlertMessage(msg: WsMessage): msg is AlertMessage {
  return !("type" in msg) && "event_id" in msg;
}

/** GET /alerts/notifications (extra endpoint): recent alerts, newest first. */
export interface AlertNotificationsResponse extends Timed {
  notifications: AlertMessage[];
}

// ---- Health ------------------------------------------------------------------

export interface HealthResponse extends Timed {
  status: "ok" | "degraded";
  db: "ok" | "unavailable";
}
