# CLAUDE.md

Geofencing & real-time alert system (MapUp assessment). Source of truth for requirements:
**docs/ASSIGNMENT.md** — re-read the relevant section before touching any endpoint.

## Repo layout (monorepo)

```
backend/            Go API + WebSocket server
  cmd/server/       main.go — wiring only (config, pool, migrations, hub, router, shutdown)
  internal/
    config/         env var parsing → Config struct
    db/             pgxpool setup, embedded migrations (migrations/*.sql via embed.FS)
    model/          domain types, ID generation, sentinel errors + model.Error
    testutil/       testutil.Pool(t): migrated pool from DATABASE_URL, skips if unset (tests only)
    repository/     hand-written SQL; the ONLY place lat/lng ↔ PostGIS conversion happens
    service/        business logic (validation, entry/exit detection, alert matching)
    handler/        HTTP handlers + router (JSON 404/405)
      respond/      respond.JSON / respond.Error / respond.Decode — the contract enforcers
    reqctx/         request start time + request ID in context; slog handler adding request_id
    realtime/       WebSocket hub (gorilla/websocket): hub goroutine owns clients; read/write pumps
    middleware/     Timing → RequestID → AccessLog → Recover → CORS (outermost first); rate limit later
frontend/           React (Vite + TS): src/{config.ts, api/{types,client,keys,hooks}.ts, components/{ui,layout,map}, pages, lib}
docs/               ASSIGNMENT, ARCHITECTURE (diagrams, EXPLAIN, edge cases), EXTRA_ENDPOINTS, DEPLOYMENT
scripts/            verify_contract.sh (curl+jq contract check: `make verify BASE_URL=…`)
docker-compose.yml  postgis + backend (+ frontend for local)
SETUP.md            required deliverable (prereqs, local setup, compose, curl guide, UI guide)
README.md           overview + video link
```

## Stack decisions

- **Go** latest stable (>=1.22). Router: stdlib `net/http` ServeMux with method+path patterns
  (`mux.HandleFunc("GET /vehicles/location/{vehicle_id}", ...)`, `r.PathValue`). No framework.
- **PostgreSQL 16 + PostGIS 3** — image `postgis/postgis:16-3.4`. Driver `jackc/pgx/v5` + `pgxpool`. No ORM.
- **Migrations**: plain `.sql` files in `embed.FS`, applied at startup, tracked in `schema_migrations`.
  Never edit an applied migration; add a new one.
- **WebSocket**: `gorilla/websocket`, hub pattern (register/unregister/broadcast, per-client
  buffered send chan + writePump/readPump, ping/pong, drop slow clients).
- **Logging**: `log/slog` JSON handler. **Config**: environment variables only (no config files).
- **Frontend**: Vite + React + TS + Tailwind CSS + react-leaflet (OSM tiles) + TanStack Query +
  react-router + sonner (toasts). API base URL and WS URL from `VITE_*` env vars.

## NON-NEGOTIABLE API contract rules (graders are strict)

1. Paths, methods and JSON field names match docs/ASSIGNMENT.md **exactly**. No `/api` prefix.
   Extra endpoints are fine (document in docs/EXTRA_ENDPOINTS.md); changing specified ones is not.
   | Method | Path | Notes |
   |---|---|---|
   | POST | `/geofences` | → `id, name, status, time_ns` |
   | GET | `/geofences?category=` | → `geofences[]` |
   | POST | `/vehicles` | → `id, vehicle_number, status, time_ns` |
   | GET | `/vehicles` | → `vehicles[]` |
   | POST | `/vehicles/location` | → `vehicle_id, location_updated, current_geofences[{geofence_id, geofence_name, status:"inside"}]` |
   | GET | `/vehicles/location/{vehicle_id}` | → `current_location{latitude,longitude,timestamp}`, `current_geofences[{geofence_id, geofence_name, category}]` |
   | POST | `/alerts/configure` | → `alert_id, geofence_id, vehicle_id, event_type, status` |
   | GET | `/alerts?geofence_id=&vehicle_id=` | → `alerts[]` |
   | GET | `/violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=` | → `violations[]`, `total_count`; limit 50, >500 clamps; +`offset` |
   | GET | `/fleet/locations` | EXTRA: all vehicles' last location + geofence ids, one query |
   | PATCH/DELETE | `/alerts/{alert_id}` | EXTRA: set status active/inactive / delete rule |
   | GET | `/alerts/notifications?limit=` | EXTRA: recent dispatched alerts (spec WS shape) |
   | WS | `/ws/alerts` | message: `event_id, event_type, timestamp, vehicle{}, geofence{}, location{}` |
2. **EVERY JSON response** (success, 400, 404, 405, 409, 429, 500) includes `"time_ns"` as a
   **string** of nanoseconds, measured from handler start to just before writing. Use one helper
   (`respond.JSON(w, r, status, &resp)`) — never call `json.NewEncoder(w)` directly in handlers.
   Unknown routes / wrong methods must also return JSON with `time_ns`.
3. **IDs**: prefixed strings `geo_`, `veh_`, `alert_`, `viol_`, `evt_` + short random lowercase id.
   Generated in Go (one function in `model`), stored as TEXT primary keys.
4. **Coordinates**: API uses `[latitude, longitude]`; PostGIS uses `(longitude, latitude)`.
   Conversion happens in ONE place in `internal/repository` and nowhere else. Geometry stored as
   `geometry(Polygon/Point, 4326)` with GiST indexes.
5. **Timestamps**: RFC3339 / ISO 8601, UTC (`t.UTC().Format(time.RFC3339)`). Store `timestamptz`.
6. **Error shape**: `{"error": "<message>", "details": <optional object>, "time_ns": "..."}`.
   Status: 400 validation, 404 not found, 409 conflict (e.g. duplicate `vehicle_number`),
   401 auth, 413 body too large, 415 wrong Content-Type, 429 rate limit, 500 internal (never leak text).
7. Enums: geofence `category` ∈ `delivery_zone|restricted_zone|toll_zone|customer_area`;
   alert `event_type` ∈ `entry|exit|both`; violation/WS `event_type` ∈ `entry|exit`.
   `vehicle_id` in `/alerts/configure` is optional → rule applies to all vehicles: `vehicle_id` AND
   `vehicle_number` are `null` in responses (frontend shows "All vehicles"). Never use "ALL".
8. Geofence validation: ≥4 points, first == last, lat ∈ [-90,90], lng ∈ [-180,180]; also reject
   self-intersecting polygons (`ST_IsValid`) with 400.

## Business logic notes

- `POST /vehicles/location`: in one transaction — store location, compute containing geofences
  (`ST_Contains`/`ST_Covers` with GiST index), diff against previous state to get entry/exit events,
  persist events as violations, update current state. Then match alert rules and broadcast
  **asynchronously** (never block the HTTP response on WS delivery). Store all alerts in DB.
- Decided: `ST_Covers` (boundary = inside); out-of-order reports go to history only (no state/events);
  timestamps >5 min in the future → 400; per-vehicle `SELECT … FOR UPDATE` serializes updates.
- Alerts: commit → `AlertDispatcher.Dispatch` (non-blocking enqueue; 4 workers) → match rules → insert
  `alert_notifications` (unique per event+rule) → `realtime.Hub.Broadcast`. WS sends the bare spec object.
- CORS enabled (frontend is deployed on a different origin). Rate limit location updates (bonus).

## Go conventions

- Layering: handler → service → repository. Handlers do decode/validate-shape/encode only.
- Pass `context.Context` as first arg everywhere; use `r.Context()`.
- Wrap errors with `fmt.Errorf("doing x: %w", err)`. Sentinel errors in `model`
  (`ErrNotFound`, `ErrConflict`, ...) + `*ValidationError`, mapped to status in ONE place (`respond.classify`).
- Define interfaces where they are consumed (service declares the repo interface it needs).
- No global state except wiring in `main`. Graceful shutdown on SIGINT/SIGTERM
  (`http.Server.Shutdown`, close hub, close pool).
- Table-driven tests; `httptest` for handlers; pure functions (validation, diffing) unit-tested.
- `gofmt`/`goimports` clean. No `panic` outside `main`.

## Frontend conventions

- Typed API client in one module (`src/api/`); types mirror the backend JSON exactly (snake_case).
- Server state via TanStack Query (invalidate on mutations); no ad-hoc fetch in components.
- One WebSocket hook with auto-reconnect (exponential backoff); msg with `type` = control, with
  `event_id` = alert → sonner toast + feed (seed feed from GET /alerts/notifications).
- Every API call shows loading and error states. Leaflet uses `[lat, lng]` — same as API.

## Commands

Go lives in `~/.local/go/bin` on this machine. Local `.env` sets `DB_HOST_PORT=5433` (system postgres owns 5432).

```bash
# full stack (repo root) — see `Makefile`
make up | down | clean (drops DB volume!) | logs | ps   # db + backend + frontend (:4173)
make seed                                  # SF demo data via the API (idempotent)
make simulate VEHICLE=CA-TRK-1001,CA-VAN-2002 ROUTE=restricted|bridge SPEED=250 INTERVAL=1s
npx wscat -c ws://localhost:8080/ws/alerts # watch alerts

# backend (from backend/)
make run DB_HOST_PORT=5433 | make check | make test-db DB_HOST_PORT=5433 | make lint

# frontend (from frontend/; Node ≥ 20.19)
npm run dev | npm run build && npm run lint | npm run format
```

Backend env vars: `DATABASE_URL` (required), `PORT` (8080), `ALLOWED_ORIGINS` ("*", comma-separated),
`LOG_LEVEL` (info), `RATE_LIMIT_RPS` (10), `RATE_LIMIT_BURST` (20), `API_KEYS` (comma-separated),
`AUTH_ENABLED` (false; requires API_KEYS). Keep this list in sync with SETUP.md.

Response helpers (internal/handler/respond):
- Response structs embed `respond.Timed` as the LAST field; write with `respond.JSON(w, r, status, &resp)`.
  Never marshal maps (encoding/json sorts keys → time_ns lands mid-body).
- Errors: `respond.Error(w, r, err)`. Return `*model.ValidationError{Message, Fields}` (→400, Fields
  become `details`) or `model.NewError(model.ErrConflict, "client-facing msg")` — its message survives
  later `%w` wrapping. Unknown errors → generic 500, logged.
- Bodies: `respond.Decode(w, r, &req)` (requires application/json → 415, 1 MiB → 413, unknown fields OK).

## Workflow rule

After each task:
1. Backend: `cd backend && go build ./... && go vet ./... && go test ./...`
2. Frontend (if touched): `cd frontend && npm run build && npm run lint`
3. Summarize what changed (files, decisions, anything deferred) and **stop** so the user can
   review and commit. Do not commit, push, or start the next task unprompted.
