# Setup

Geofencing & real-time vehicle alerts: Go API + PostGIS + WebSocket alerts, React frontend.
Architecture and design decisions: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) ·
Deployment: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md).

- [Prerequisites](#prerequisites)
- [Run everything with Docker Compose](#run-everything-with-docker-compose)
- [Local development without Docker](#local-development-without-docker)
- [Demo data and the simulator](#demo-data-and-the-simulator)
- [Environment variables](#environment-variables)
- [API testing guide](#api-testing-guide)
- [Frontend usage guide](#frontend-usage-guide)
- [Tests and checks](#tests-and-checks)

## Prerequisites

| Tool | Version | Needed for |
|---|---|---|
| Docker + Docker Compose v2 | Docker 24+ | everything via `make up` |
| Go | **1.27** (go.mod: `go 1.27.2`) | backend without Docker, backend tests |
| Node.js + npm | **20.19+** or 22.12+ | frontend without Docker |
| curl, jq | any | API guide, `scripts/verify_contract.sh` |
| websocat *or* npx wscat | any | watching `/ws/alerts` from a terminal |

## Run everything with Docker Compose

```bash
cp .env.example .env          # optional; every value has a default
make up                       # = docker compose up -d --build --wait (db, backend, frontend)
```

| Service | URL |
|---|---|
| Frontend | <http://localhost:4173> |
| API | <http://localhost:8080> (try `curl localhost:8080/health`) |
| PostgreSQL + PostGIS | `localhost:5432` (user/password/db: `geofence`) |

Migrations run automatically when the backend starts. Other targets: `make logs`, `make ps`,
`make down` (keeps data), `make clean` (**deletes** the database volume).

> Port 5432 already used by a local PostgreSQL? Set `DB_HOST_PORT=5433` in `.env`.
> Port 3000 is not used; the frontend is on 4173.

## Local development without Docker

You still need PostgreSQL 16 with PostGIS 3. The quickest is the compose database alone:

```bash
docker compose up -d db
```

(Or a local install: create a database and a user that may run `CREATE EXTENSION postgis`.)

**Backend** (Go 1.27):

```bash
cd backend
make run                                  # DATABASE_URL defaults to the compose db on localhost:5432
# or: DATABASE_URL='postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable' go run ./cmd/server
```

**Frontend** (Node 20.19+):

```bash
cd frontend
npm ci
npm run dev                               # http://localhost:5173, talks to http://localhost:8080
```

Point it elsewhere with `frontend/.env.local`: `VITE_API_URL=http://host:port`.

## Demo data and the simulator

The demo dataset is in San Francisco (matching the assignment examples): 5 geofences covering
all four categories (including the **Port Terminal Restricted Area**), 4 vehicles and 4 alert
rules, including an *all vehicles, entry and exit* rule on the restricted zone.

```bash
make seed                                 # through the API; idempotent (re-running changes nothing)
make simulate                             # drives CA-TRK-1001 through every zone (Ctrl-C to stop)
make simulate VEHICLE=CA-TRK-1001,CA-VAN-2002 SPEED=500    # two vehicles, faster
make simulate ROUTE=bridge                # through the Bay Bridge toll plaza
```

Without Docker: `cd backend && go run ./cmd/seed` and `go run ./cmd/simulator -help`.

| Simulator flag | Default | Meaning |
|---|---|---|
| `-base-url` | `http://localhost:8080` (or `$API_URL`) | API to post to |
| `-vehicle` | `CA-TRK-1001` | comma-separated vehicle numbers or `veh_…` ids |
| `-route` | `restricted` | `restricted` (Mission → Downtown → SoMa → port terminal) or `bridge` |
| `-interval` | `1s` | time between location updates |
| `-speed` | `250` | km/h; distance per update = speed × interval (high = time-lapse) |
| `-stagger` | `3s` | start delay between vehicles |
| `-loop` | off | drive back and forth until interrupted |

Lines marked `*` in its output are updates where the vehicle's set of geofences changed.

## Environment variables

**Backend** (environment only; no config files). Invalid values stop the server at startup with
a message naming each problem.

| Variable | Default | Description |
|---|---|---|
| `DATABASE_URL` | — (**required**) | PostgreSQL URL, e.g. `postgres://user:pass@host:5432/db?sslmode=disable` |
| `PORT` | `8080` | HTTP listen port |
| `ALLOWED_ORIGINS` | `*` | Comma-separated exact origins allowed by CORS **and** the WebSocket origin check |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` (JSON logs to stdout) |
| `RATE_LIMIT_RPS`, `RATE_LIMIT_BURST` | `10`, `20` | Validated but **not yet enforced**: rate limiting is not implemented |
| `AUTH_ENABLED`, `API_KEYS` | `false`, — | Validated but **not yet enforced**: API-key auth is not implemented |

**Docker Compose** (`.env`, see `.env.example`): `POSTGRES_USER`/`POSTGRES_PASSWORD`/`POSTGRES_DB`
(default `geofence`), `DB_HOST_PORT` (5432), `BACKEND_PORT` (8080), `FRONTEND_PORT` (4173), and
the backend variables above.

**Frontend** (build time, inlined by Vite):

| Variable | Default | Description |
|---|---|---|
| `VITE_API_URL` | `http://localhost:8080` | API base URL (no trailing slash) |
| `VITE_WS_URL` | API URL with `ws(s)://…/ws/alerts` | Override the WebSocket URL |
| `VITE_API_KEY` | — | Sent as `X-API-Key` if set (for when auth is added) |

## API testing guide

Every response, including errors, carries `"time_ns"` (handler time in nanoseconds, as a
string). Errors look like `{"error": "...", "details": {...}, "time_ns": "..."}`. Coordinates are
`[latitude, longitude]`; timestamps are RFC 3339 UTC. Full contract: [docs/ASSIGNMENT.md](docs/ASSIGNMENT.md).

The commands below run in order in one shell (they capture ids with `jq`). Automated version:
`make verify` (or `BASE_URL=… scripts/verify_contract.sh`) checks every endpoint and prints a
PASS/FAIL table.

```bash
API=http://localhost:8080
```

### 1. POST /geofences

```bash
GEO=$(curl -s -X POST $API/geofences -H 'Content-Type: application/json' -d '{
  "name": "Downtown Restricted Zone",
  "description": "Guide example",
  "coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
  "category": "restricted_zone"
}')
echo "$GEO"            # {"id":"geo_…","name":"Downtown Restricted Zone","status":"active","time_ns":"…"}
GEO_ID=$(jq -r .id <<<"$GEO")
```

Validation (400 with per-field `details`): open polygon, fewer than 4 points, lat outside ±90,
lng outside ±180, unknown category, more than 1000 points, self-intersecting shape:

```bash
curl -s -X POST $API/geofences -H 'Content-Type: application/json' \
  -d '{"name":"Bad","coordinates":[[1,1],[1,2],[2,2],[2,1]],"category":"toll_zone"}'
# {"error":"invalid geofence","details":{"coordinates":"polygon must be closed: first and last points must be identical"},"time_ns":"…"}
```

### 2. GET /geofences

```bash
curl -s "$API/geofences" | jq '.geofences | length'
curl -s "$API/geofences?category=restricted_zone" | jq '.geofences[] | {id, name, category, created_at}'
```

### 3. POST /vehicles

```bash
VEH=$(curl -s -X POST $API/vehicles -H 'Content-Type: application/json' -d '{
  "vehicle_number": "KA-01-AB-1234",
  "driver_name": "John Doe",
  "vehicle_type": "truck",
  "phone": "+1234567890"
}')
echo "$VEH"            # {"id":"veh_…","vehicle_number":"KA-01-AB-1234","status":"active","time_ns":"…"}
VEH_ID=$(jq -r .id <<<"$VEH")
# Already registered? Look it up instead:
[ "$VEH_ID" = null ] && VEH_ID=$(curl -s $API/vehicles | jq -r '.vehicles[] | select(.vehicle_number=="KA-01-AB-1234") | .id')
```

Registering the same `vehicle_number` again returns **409**
`{"error":"vehicle_number \"KA-01-AB-1234\" is already registered",…}`.

### 4. GET /vehicles

```bash
curl -s $API/vehicles | jq '.vehicles[] | {id, vehicle_number, driver_name, status}'
```

### 5. POST /alerts/configure

```bash
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO_ID\",\"vehicle_id\":\"$VEH_ID\",\"event_type\":\"entry\"}"
# {"alert_id":"alert_…","geofence_id":"geo_…","vehicle_id":"veh_…","event_type":"entry","status":"active","time_ns":"…"}

# Omit vehicle_id for a rule covering every vehicle (returned as "vehicle_id": null):
curl -s -X POST $API/alerts/configure -H 'Content-Type: application/json' \
  -d "{\"geofence_id\":\"$GEO_ID\",\"event_type\":\"exit\"}"
```

An identical active rule returns **409** with `details.existing_alert_id`.

### 6. Watch alerts on /ws/alerts (second terminal)

```bash
websocat ws://localhost:8080/ws/alerts
# or: npx wscat -c ws://localhost:8080/ws/alerts
# → {"type":"connected",…} then one JSON alert per line, exactly in the assignment's format
```

### 7. POST /vehicles/location (outside → inside → outside)

```bash
post() { curl -s -X POST $API/vehicles/location -H 'Content-Type: application/json' \
  -d "{\"vehicle_id\":\"$VEH_ID\",\"latitude\":$1,\"longitude\":$2,\"timestamp\":\"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"; echo; sleep 1; }
post 37.7700 -122.4300     # outside
post 37.7799 -122.4144     # inside → entry event; the websocat terminal receives the alert
post 37.7700 -122.4300     # outside → exit event (alert via the all-vehicles exit rule)
```

Response: `{"vehicle_id":"veh_…","location_updated":true,"current_geofences":[{"geofence_id":"geo_…","geofence_name":"…","status":"inside"}],"time_ns":"…"}`.
Errors: unknown vehicle → **404**; lat 91, missing fields or a timestamp more than 5 minutes
ahead → **400**.

### 8. GET /vehicles/location/{vehicle_id}

```bash
curl -s $API/vehicles/location/$VEH_ID | jq
# {"vehicle_id":…,"vehicle_number":…,"current_location":{"latitude":…,"longitude":…,"timestamp":…},"current_geofences":[…],"time_ns":…}
```

`current_location` is `null` for a vehicle that has never reported.

### 9. GET /alerts

```bash
curl -s "$API/alerts?geofence_id=$GEO_ID" | jq '.alerts[] | {alert_id, vehicle_number, event_type, status}'
curl -s "$API/alerts?vehicle_id=$VEH_ID"  | jq '.alerts | length'
```

All-vehicle rules have `"vehicle_id": null, "vehicle_number": null`.

### 10. GET /violations/history

```bash
curl -s "$API/violations/history?vehicle_id=$VEH_ID&limit=10" | jq '{total_count, events: [.violations[] | {event_type, geofence_name, timestamp}]}'
curl -s "$API/violations/history?geofence_id=$GEO_ID&start_date=2025-01-01T00:00:00Z&end_date=2030-01-01T00:00:00Z&limit=50&offset=0" | jq .total_count
```

`limit` defaults to 50; values above 500 are clamped to 500; `total_count` ignores
`limit`/`offset`. Bad dates or `start_date` after `end_date` → **400**.

### Extra endpoints (additions, not in the assignment)

The nine specified endpoints are unchanged; these are additive. Details:
[docs/EXTRA_ENDPOINTS.md](docs/EXTRA_ENDPOINTS.md).

```bash
curl -s $API/health                                     # liveness + DB check (503 when the DB is down)
curl -s $API/fleet/locations | jq '.vehicles[0]'        # every vehicle's last position + geofence ids, one query
curl -s "$API/alerts/notifications?limit=5" | jq '.notifications[0]'   # recent dispatched alerts (WS format)
RULE=$(curl -s "$API/alerts?vehicle_id=$VEH_ID" | jq -r '.alerts[0].alert_id')
curl -s -X PATCH $API/alerts/$RULE -H 'Content-Type: application/json' -d '{"status":"inactive"}' | jq '{alert_id, status}'
curl -s -X DELETE $API/alerts/$RULE                     # {"alert_id":"alert_…","deleted":true,"time_ns":"…"}
```

## Frontend usage guide

Open <http://localhost:4173> (Docker) or <http://localhost:5173> (dev server). The pill in the
top-right shows the live-alert connection: **green Live**, amber reconnecting, red offline.

**Draw a geofence** (*Geofences* → **New geofence**)
1. Enter a name, optional description and a category.
2. *Draw on map*: click the map to place vertices (live preview), **Undo point** / **Clear** as
   needed, then click the first vertex or **Finish** to close the polygon.
   *Or* *Enter coordinates*: paste `[[lat,lng],…]` JSON or one `lat, lng` per line and press
   **Preview on map**. Both views edit the same polygon.
3. **Create geofence**. Mistakes are shown next to the field (e.g. "polygon must be closed",
   or PostGIS's "Self-intersection at [lat, lng]").
4. The list beside the map filters by category (the filter is in the URL); hover an item to
   highlight its polygon, click to zoom to it.

**Register a vehicle and update its location** (*Vehicles*)
1. **Register vehicle** → number, driver, type (or *Other…*), phone. A duplicate number is
   flagged on the field.
2. In *Update location*, pick the vehicle (the map flies to it), then **click the map** to set
   the position or type latitude/longitude. The timestamp defaults to now (**Now** resets it).
3. **Update location**: the marker moves immediately, the result shows the geofences it is
   now inside, and toasts announce "entered"/"left". The table below shows every vehicle's
   position and geofences; **Details** opens the vehicle's current location from the API.
4. The same panel is on the *Dashboard*, next to a *Live alerts* tab.

**Configure alerts** (*Alerts*)
1. Pick a geofence, a vehicle or **All vehicles**, and *Entry*, *Exit* or *Entry & exit*.
2. The rules table filters by geofence and vehicle; the switch pauses/resumes a rule; the bin
   icon deletes it (with confirmation).
3. *Live alerts* lists incoming alerts as they happen (red for restricted-zone entries); click
   one to see it on the Dashboard map. Each alert also pops a toast on every page.

**Read history** (*History*)
1. Filter by vehicle, geofence and a date-time range; choose 25/50/100/500 per page. All
   filters live in the URL, so a view can be shared or bookmarked.
2. Page through with the numbers or Previous/Next ("Showing 51–100 of 245").
3. Click a row to see the event's point and geofence on the mini map; **Export page (CSV)**
   downloads the current page.

## Tests and checks

```bash
make test                                 # backend: build + vet + unit tests
make test-db                              # + PostGIS integration tests (needs the compose db)
cd backend && go test -race ./...         # race detector (set DATABASE_URL to include integration tests)
cd frontend && npm test && npm run lint && npm run build
make verify                               # API contract check against BASE_URL (default localhost:8080)
```
