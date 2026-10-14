CREATE EXTENSION IF NOT EXISTS postgis;

-- Geofences: boundary stored as (longitude, latitude) per PostGIS convention.
CREATE TABLE geofences (
    id          text PRIMARY KEY,
    name        text NOT NULL,
    description text,
    category    text NOT NULL
        CHECK (category IN ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    boundary    geometry(Polygon, 4326) NOT NULL CHECK (ST_IsValid(boundary)),
    status      text NOT NULL DEFAULT 'active',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX geofences_boundary_gist ON geofences USING GIST (boundary);
CREATE INDEX geofences_category_idx  ON geofences (category);

CREATE TABLE vehicles (
    id               text PRIMARY KEY,
    vehicle_number   text NOT NULL UNIQUE,
    driver_name      text NOT NULL,
    vehicle_type     text NOT NULL,
    phone            text NOT NULL,
    status           text NOT NULL DEFAULT 'active',
    last_latitude    double precision,
    last_longitude   double precision,
    last_location_at timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now()
);

-- Full location history (append-only).
CREATE TABLE vehicle_locations (
    id          bigserial PRIMARY KEY,
    vehicle_id  text NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    location    geometry(Point, 4326) NOT NULL,
    recorded_at timestamptz NOT NULL,
    received_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX vehicle_locations_vehicle_recorded_idx ON vehicle_locations (vehicle_id, recorded_at DESC);
CREATE INDEX vehicle_locations_location_gist        ON vehicle_locations USING GIST (location);

-- Geofences each vehicle is currently inside; diffed on every location update
-- to detect entry/exit events.
CREATE TABLE vehicle_geofence_state (
    vehicle_id  text NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    geofence_id text NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    entered_at  timestamptz NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);
CREATE INDEX vehicle_geofence_state_geofence_idx ON vehicle_geofence_state (geofence_id);

-- Alert rules; vehicle_id NULL means the rule applies to all vehicles.
CREATE TABLE alert_rules (
    id          text PRIMARY KEY,
    geofence_id text NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    vehicle_id  text REFERENCES vehicles (id) ON DELETE CASCADE,
    event_type  text NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status      text NOT NULL DEFAULT 'active',
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX alert_rules_geofence_idx ON alert_rules (geofence_id);
CREATE INDEX alert_rules_vehicle_idx  ON alert_rules (vehicle_id);

-- Entry/exit events (served by GET /violations/history). IDs are 'viol_...'.
CREATE TABLE geofence_events (
    id          text PRIMARY KEY,
    vehicle_id  text NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    geofence_id text NOT NULL REFERENCES geofences (id) ON DELETE CASCADE,
    event_type  text NOT NULL CHECK (event_type IN ('entry', 'exit')),
    location    geometry(Point, 4326) NOT NULL,
    latitude    double precision NOT NULL,
    longitude   double precision NOT NULL,
    occurred_at timestamptz NOT NULL
);
CREATE INDEX geofence_events_vehicle_occurred_idx  ON geofence_events (vehicle_id, occurred_at DESC);
CREATE INDEX geofence_events_geofence_occurred_idx ON geofence_events (geofence_id, occurred_at DESC);
CREATE INDEX geofence_events_occurred_idx          ON geofence_events (occurred_at DESC);

-- Every alert dispatched over the real-time channel. IDs are 'evt_...'.
-- The rule may be deleted later; the notification record is kept.
CREATE TABLE alert_notifications (
    id                text PRIMARY KEY,
    alert_rule_id     text REFERENCES alert_rules (id) ON DELETE SET NULL,
    geofence_event_id text NOT NULL REFERENCES geofence_events (id) ON DELETE CASCADE,
    payload           jsonb NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX alert_notifications_event_idx   ON alert_notifications (geofence_event_id);
CREATE INDEX alert_notifications_created_idx ON alert_notifications (created_at DESC);
