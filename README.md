# Geofence Tracker: Geofencing & Real-time Alert System

A full-stack system for defining geofences, tracking vehicles and alerting in real time. Draw
virtual boundaries (delivery, restricted, toll and customer zones) on a map or paste their
coordinates; register vehicles and stream their positions; configure alert rules per geofence
and vehicle (or for all vehicles); and watch entry/exit alerts arrive in the browser within a
second over WebSocket, with full, filterable history. The backend is Go (standard library HTTP,
pgx) on PostgreSQL 16 + PostGIS: each location update is one transaction that locks the
vehicle, finds containing geofences with an indexed `ST_Covers` query, diffs against the
previous state and records entry/exit events, then hands them to a worker pool that matches
rules and broadcasts alerts without delaying the HTTP response. The frontend is React +
TypeScript (Vite, Tailwind, react-leaflet, TanStack Query).

| | |
|---|---|
| **Live frontend** | https://frontend-gdlp23oyn-amanpreet116s-projects.vercel.app/geofences * |
| **Backend API** | <https://geofence-tracker-backend-production.up.railway.app> (try [`/health`](https://geofence-tracker-backend-production.up.railway.app/health), [`/geofences`](https://geofence-tracker-backend-production.up.railway.app/geofences)) |
| **WebSocket** | `wss://geofence-tracker-backend-production.up.railway.app/ws/alerts` |
| **Docker Hub image** | [`amanpreet116/geofence-tracker-backend:v1.0.0`](https://hub.docker.com/r/amanpreet116/geofence-tracker-backend) (linux/amd64 + linux/arm64) |
| **Video walkthrough** | *(link to be added)* |

![Dashboard: fleet map with geofences, live alerts panel](docs/images/dashboard.png)

## Quick start

```bash
make up        # PostGIS + backend + frontend via Docker Compose → http://localhost:4173 (API :8080)
make seed      # San Francisco demo data (5 geofences, 4 vehicles, 4 alert rules), idempotent
make simulate  # drive vehicles through the restricted zone and watch the alerts
make verify    # contract check: 37 checks against BASE_URL (default http://localhost:8080)
```

Or run only the published backend image against any PostGIS database:

```bash
docker run -p 8080:8080 -e DATABASE_URL='postgres://user:pass@host:5432/db?sslmode=require' \
  amanpreet116/geofence-tracker-backend:v1.0.0
```

Everything else (prerequisites, running without Docker, environment variables, step-by-step
curl testing, WebSocket testing and a frontend guide) is in **[SETUP.md](SETUP.md)**.

## API reference

Base URL: `http://localhost:8080` locally, or the Railway URL above. No `/api` prefix.

**Conventions (every endpoint)**

- Every JSON response, errors included, has `"time_ns"`: handler time in nanoseconds, **as a string**.
- Errors: `{"error": "message", "details": {…optional per-field info}, "time_ns": "…"}`.
  Status codes: `400` validation, `404` not found, `405` wrong method, `409` conflict,
  `413` body over 1 MiB, `415` non-JSON body, `500` internal (no details leaked),
  `503` + `Retry-After` when the database is unavailable.
- Coordinates are `[latitude, longitude]`; timestamps are RFC 3339 in UTC.
- IDs are prefixed strings: `geo_`, `veh_`, `alert_`, `viol_`, `evt_`.
- CORS is enabled for the origins in `ALLOWED_ORIGINS`.

### Specified endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/geofences` | Create a geofence |
| `GET` | `/geofences?category=` | List geofences, optionally by category |
| `POST` | `/vehicles` | Register a vehicle |
| `GET` | `/vehicles` | List vehicles |
| `POST` | `/vehicles/location` | Report a position; detects entry/exit, triggers alerts |
| `GET` | `/vehicles/location/{vehicle_id}` | A vehicle's current location and geofences |
| `POST` | `/alerts/configure` | Create an alert rule |
| `GET` | `/alerts?geofence_id=&vehicle_id=` | List alert rules |
| `GET` | `/violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=` | Entry/exit event log |
| `WS` | `/ws/alerts` | Real-time alert stream |

#### `POST /geofences`

```json
// request
{
  "name": "Downtown Restricted Zone",
  "description": "Optional",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "restricted_zone"
}
// 201
{"id": "geo_…", "name": "Downtown Restricted Zone", "status": "active", "time_ns": "…"}
```

`category` ∈ `delivery_zone | restricted_zone | toll_zone | customer_area`. The polygon needs
at least 4 points, first = last, lat ∈ [-90, 90], lng ∈ [-180, 180], at most 1000 points, and must
not self-intersect (checked with PostGIS `ST_IsValid`). Any violation → `400` with
`details.coordinates`.

#### `GET /geofences?category=restricted_zone`

`200 {"geofences": [{"id", "name", "description", "coordinates", "category", "created_at"}], "time_ns"}`.
Unknown category → `400`.

#### `POST /vehicles`

```json
// request
{"vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe", "vehicle_type": "truck", "phone": "+1234567890"}
// 201
{"id": "veh_…", "vehicle_number": "KA-01-AB-1234", "status": "active", "time_ns": "…"}
```

Vehicle numbers are normalized (uppercase, single spaces); a duplicate → `409`.

#### `GET /vehicles`

`200 {"vehicles": [{"id", "vehicle_number", "driver_name", "vehicle_type", "phone", "status", "created_at"}], "time_ns"}`.

#### `POST /vehicles/location`

```json
// request
{"vehicle_id": "veh_…", "latitude": 37.7799, "longitude": -122.4144, "timestamp": "2025-01-15T10:35:00Z"}
// 200
{
  "vehicle_id": "veh_…",
  "location_updated": true,
  "current_geofences": [{"geofence_id": "geo_…", "geofence_name": "Downtown Restricted Zone", "status": "inside"}],
  "time_ns": "…"
}
```

One transaction per update: lock the vehicle row, store the point, find covering geofences
(a point on the boundary counts as inside), diff against the previous set, record entry/exit
events, commit. Alerts are matched and broadcast afterwards, so the response never waits on
them. Reports older than the vehicle's latest are kept in history only, exact duplicates are
no-ops, and timestamps more than 5 minutes in the future → `400`. Unknown vehicle → `404`.

#### `GET /vehicles/location/{vehicle_id}`

```json
{
  "vehicle_id": "veh_…",
  "vehicle_number": "KA-01-AB-1234",
  "current_location": {"latitude": 37.7799, "longitude": -122.4144, "timestamp": "2025-01-15T10:35:00Z"},
  "current_geofences": [{"geofence_id": "geo_…", "geofence_name": "Downtown Restricted Zone", "category": "restricted_zone"}],
  "time_ns": "…"
}
```

`current_location` is `null` for a vehicle that has never reported.

#### `POST /alerts/configure`

```json
// request (omit vehicle_id for a rule covering all vehicles)
{"geofence_id": "geo_…", "vehicle_id": "veh_…", "event_type": "entry"}
// 201
{"alert_id": "alert_…", "geofence_id": "geo_…", "vehicle_id": "veh_…", "event_type": "entry", "status": "active", "time_ns": "…"}
```

`event_type` ∈ `entry | exit | both`. All-vehicle rules return `"vehicle_id": null`. An
identical active rule → `409` with `details.existing_alert_id`.

#### `GET /alerts?geofence_id=&vehicle_id=`

`200 {"alerts": [{"alert_id", "geofence_id", "geofence_name", "vehicle_id", "vehicle_number", "event_type", "status", "created_at"}], "time_ns"}`.
For all-vehicle rules, `vehicle_id` and `vehicle_number` are `null`.

#### `GET /violations/history`

```json
{
  "violations": [{
    "id": "viol_…", "vehicle_id": "veh_…", "vehicle_number": "KA-01-AB-1234",
    "geofence_id": "geo_…", "geofence_name": "Downtown Restricted Zone",
    "event_type": "entry", "latitude": 37.7799, "longitude": -122.4144, "timestamp": "2025-01-15T10:35:00Z"
  }],
  "total_count": 1,
  "time_ns": "…"
}
```

All filters optional. `limit` defaults to 50 and values above 500 are clamped to 500; `offset`
pages through results; `total_count` ignores `limit`/`offset`. Invalid dates or
`start_date` after `end_date` → `400`. Newest first.

#### `WS /ws/alerts`

On connect the server sends `{"type":"connected", …}`; each alert is then sent as the bare
object from the assignment:

```json
{
  "event_id": "evt_…",
  "event_type": "entry",
  "timestamp": "2025-01-15T10:35:00Z",
  "vehicle": {"vehicle_id": "veh_…", "vehicle_number": "KA-01-AB-1234", "driver_name": "John Doe"},
  "geofence": {"geofence_id": "geo_…", "geofence_name": "Downtown Restricted Zone", "category": "restricted_zone"},
  "location": {"latitude": 37.7799, "longitude": -122.4144}
}
```

Messages with `type` are control messages (`connected`, `pong`); messages with `event_id` are
alerts. The server pings every 30 s; clients may send `{"type":"ping"}`.

```bash
npx wscat -c ws://localhost:8080/ws/alerts
```

### Extra endpoints (additions, details in [docs/EXTRA_ENDPOINTS.md](docs/EXTRA_ENDPOINTS.md))

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness + DB check: `{"status":"ok","db":"ok"}`, `503` when the DB is down |
| `GET` | `/fleet/locations` | Every vehicle's last location + current geofence ids in one query (map view) |
| `PATCH` | `/alerts/{alert_id}` | Body `{"status":"active"\|"inactive"}`: pause or resume a rule |
| `DELETE` | `/alerts/{alert_id}` | Delete a rule: `{"alert_id":"…","deleted":true}` |
| `GET` | `/alerts/notifications?limit=` | Recently dispatched alerts in the WebSocket format (default 50, max 200) |

## Frontend

React app with five pages:

- **Dashboard**: fleet map with geofences colored by category, vehicle markers, live alert feed,
  connection indicator.
- **Geofences**: draw polygons on the map (undo, close on the first point) or paste
  coordinates; category filter in the URL.
- **Vehicles**: register vehicles, click the map to update a location (optimistic move), and a
  details drawer with current geofences.
- **Alerts**: create rules (one vehicle or all vehicles; entry, exit or both), pause, resume and
  delete them.
- **History**: filter by vehicle, geofence and date range, with server-side pagination,
  shareable URLs, a mini map per event and CSV export.

The header also has a recent-alerts bell and a light/dark theme switcher. Alerts arrive as
toasts (red for restricted zones) with a pulse on the map. The socket reconnects with
exponential backoff and backfills missed alerts.

## Documentation

- [SETUP.md](SETUP.md): setup, environment variables, API testing guide, frontend usage
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): components, location-update sequence, schema and
  index reasoning, design decisions, measured query plans, edge cases, known limitations
- [docs/EXTRA_ENDPOINTS.md](docs/EXTRA_ENDPOINTS.md): additions beyond the spec and the WebSocket protocol
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md): Docker Hub, backend hosting, PostGIS database, Vercel
- [docs/ASSIGNMENT.md](docs/ASSIGNMENT.md): the original brief

## Deployment

| Part | Where |
|---|---|
| Backend image | Docker Hub `amanpreet116/geofence-tracker-backend` (`v1.0.0`, `latest`; amd64 + arm64, distroless, ~28 MB) |
| Backend | Railway, running the Docker Hub image; migrations run at startup |
| Database | Supabase PostgreSQL with PostGIS (session pooler) |
| Frontend | Vercel (`frontend/`, SPA rewrites in `vercel.json`) |

`scripts/verify_contract.sh` passes all 37 checks against the production backend
(`make verify BASE_URL=https://geofence-tracker-backend-production.up.railway.app`).

## Highlights

- All nine specified endpoints with the exact paths, fields and `time_ns` on every response,
  including errors; `scripts/verify_contract.sh` checks the whole contract (37 checks) and the
  WebSocket alert shape against any deployment.
- Correct under concurrency: per-vehicle row locking, tested with 20 parallel updates.
- Boundary points count as inside; overlapping/nested zones; out-of-order and duplicate reports
  handled; database outages return 503 and recover without a restart.
- Real-time: a bounded worker pool dispatches alerts (never blocking location updates),
  notifications are deduplicated per (event, rule), and slow WebSocket clients are dropped
  instead of stalling others.
- Tests: Go unit + PostGIS integration tests (race detector clean), Vitest for the frontend's
  validation and socket logic.

## Not implemented yet

Rate limiting and API-key authentication (bonus items): the config variables, the 401/429
responses and the frontend's `X-API-Key` header are in place, but the middleware is not. No
update/delete endpoints for geofences and vehicles. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
for the known limitations and roadmap.
