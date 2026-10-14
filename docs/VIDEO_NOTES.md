# Video notes (private outline, not graded)

Target 30–45 min: **Demo 10–12 · Code 12–18 · Gaps & roadmap 10–15.**

Before recording: `make clean && make up && make seed`. Two terminals (simulator, websocat),
the browser at <http://localhost:4173> with DevTools closed, editor open on the repo.
Zoom the browser to 110% and the terminal font up.

---

## 1. Demo script (10–12 min)

| # | Do | Say / point out |
|---|---|---|
| 1 | `make up` (already up: show `make ps`, three services healthy) | One command: PostGIS, Go API, nginx-served React. Migrations run at startup. |
| 2 | `make seed` (run it **twice**) | Seeds through the public API, so it doubles as a smoke test; second run shows `=` everywhere (idempotent). |
| 3 | Browser → **Dashboard** | Header pill **Live** (WebSocket). Stats, map with zones in category colors; "N vehicles without a location" hint. |
| 4 | **Geofences** → *New geofence* → name "Pier 39 Restricted", category *Restricted zone* → click 5 points on the map near Pier 39 → **Undo point** once → click the first vertex to close | Live preview; first vertex closes the ring; drawing uses `useMapEvents`, no plugin. |
| 5 | Switch to *Enter coordinates* | Same polygon as text, kept in sync; JSON or `lat, lng` lines. |
| 6 | Paste a bowtie `[[37.80,-122.44],[37.81,-122.43],[37.81,-122.44],[37.80,-122.43],[37.80,-122.44]]` → **Preview** → **Create** | Client rules pass; PostGIS rejects: "Self-intersection at [37.805, -122.435]" next to the field. Then fix it (use the drawn one) → **Create** → toast, list selects + map flies to it. |
| 7 | Category filter → *Restricted zone*; note `?category=` in the URL; hover / click items | Filter is server-side and shareable; hover highlights, click flies. |
| 8 | **Vehicles** → *Register vehicle* "ca-dem-0001" (shows "Will be saved as CA-DEM-0001"), then the same number again | Normalization; 409 shown on the field. |
| 9 | Select it in *Update location*, click the map inside Downtown → **Update location** | Marker moves immediately (optimistic), result badges, toast "entered Downtown Delivery Zone". **Details** drawer = `GET /vehicles/location/{id}`. |
| 10 | **Alerts** → rule: *Pier 39 Restricted* · *All vehicles* · *Entry & exit* → create; create again | 409 duplicate; the rules table shows "All vehicles" for `vehicle_id: null`; toggle a rule off/on. |
| 11 | Terminal A: `websocat ws://localhost:8080/ws/alerts` (or `npx wscat -c …`) | Hello message, then raw alerts, exactly the spec's JSON. |
| 12 | Terminal B: `make simulate VEHICLE=CA-TRK-1001,CA-VAN-2002 SPEED=400` · switch to the **Dashboard** | Two vehicles drive Mission → Downtown → SoMa → **Port Terminal Restricted Area**. Red toasts on restricted entry, map pulse rings, *Live alerts* tab badge, websocat prints the same alerts. Click an alert → map flies there. |
| 13 | **History** → vehicle *CA-TRK-1001*, per page 25, page 2; copy the URL into a new tab | Server pagination "Showing 26–50 of N", URL-synced filters restore in the new tab; click a row → mini map; **Export page (CSV)**. Set *From* after *To* → inline error. |
| 14 | Resilience (30 s): `docker compose stop backend` → pill goes amber → red; `docker compose start backend` → back to **Live**; alerts missed meanwhile appear in the feed | Backoff + jitter, heartbeat, history backfill on reconnect. |
| 15 | `make verify` | 37 contract checks incl. a WebSocket listener, all PASS. |

## 2. Code walkthrough order (12–18 min)

Open in this order; 2–3 points each.

1. **`docs/ARCHITECTURE.md`** (component diagram + sequence diagram on screen)
   - Layers: handler → service → repository; dispatcher → hub on the side.
   - The one transaction per location update; alerts after commit.
2. **`backend/cmd/server/main.go`**
   - Wiring only, no globals; middleware order (timing outermost so even panics get `time_ns`).
   - Shutdown order: HTTP → drain alert queue → close WebSocket clients → DB pool.
3. **`backend/internal/handler/respond/respond.go`**
   - `Timed` embedded last → `time_ns` on every response, compile-time enforced; why not a map
     (key order).
   - `classify`: the single error → status map (validation 400 with `details`, 409, 503 for DB
     outages, generic 500).
4. **`backend/internal/repository/geo.go`**
   - The only `[lat, lng]` ↔ `(lng lat)` conversion; binary float arrays (exact round-trip);
     PostGIS reason rewritten to `[lat, lng]`.
5. **`backend/internal/service/location.go`** (the core)
   - `SELECT … FOR UPDATE` and why (the race: two updates diffing the same "before" state).
   - Duplicate / out-of-order rules; `ComputeTransitions` is pure (show its table test).
   - `EventDispatcher` interface: handler never waits.
6. **`backend/internal/repository/location.go`**
   - `ST_Covers` (boundary = inside) + GiST; batch writes for events and state.
7. **`backend/internal/service/location_integration_test.go`**
   - `TestConcurrentUpdates`: 20 parallel updates, replay-based expectation; mention it fails
     without the row lock (I verified that by removing it).
8. **`backend/internal/service/alert_dispatcher.go`** + **`alert.go`**
   - Bounded queue + 4 workers; overflow = backpressure, not loss; drain on shutdown.
   - `CandidateRules` (one query per batch) + pure `RuleMatches`; dedupe via unique index.
9. **`backend/internal/realtime/hub.go`** + `client.go`
   - One goroutine owns the client set (no mutexes); slow clients dropped; ping/pong; origin
     check; bare spec-shaped alert messages.
10. **`backend/internal/db/migrations/0001_init.sql`** (+ 0002, 0003)
    - Schema, CHECKs, partial unique index for active rules; index reasoning table in
      ARCHITECTURE.md; EXPLAIN results (top-N index scans).
11. **`frontend/src/api/{types,client,hooks}.ts`**
    - Types mirror the JSON exactly; `ApiError.fieldErrors` → inline errors; optimistic
      location update with rollback + reconcile.
12. **`frontend/src/realtime/alertSocket.ts`** + `AlertSocketProvider.tsx`
    - Framework-free socket class (tested with fake timers): backoff + jitter, focus/online
      retry, heartbeat; StrictMode-safe deferred connect; dedupe by `event_id`.
13. **`frontend/src/components/map/DrawLayer.tsx`** + `features/geofences/GeofenceForm.tsx`
    - Click-to-draw with `useMapEvents`; one points array shared by draw and text modes;
      validation mirrors the backend keys so server errors land on the same fields.
14. **`frontend/src/pages/HistoryPage.tsx`** + `lib/history.ts`
    - URL as state; `keepPreviousData`; the datetime round-trip bug I found and fixed
      (`DateTimeFilter`).

## 3. Gaps, improvements & roadmap (10–15 min)

### Not done (with estimates)

| Item | Estimate | Notes |
|---|---|---|
| Rate limiting on `POST /vehicles/location` (bonus) | 0.5 day | Token bucket per API key / IP (`x/time/rate`); config already parsed; 429 already mapped. |
| API-key auth (bonus) → JWT + RBAC | 0.5 day keys · 2–3 days JWT/RBAC | Middleware + `X-API-Key` (frontend already sends it); roles: viewer / dispatcher / admin. |
| Pagination on all list endpoints (bonus) | 0.5 day | Keyset (`created_at, id`) cursors; history already has `limit/offset`. |
| Edit/delete geofences and vehicles | 1 day | Schema cascades exist; needs endpoints + UI; decide what happens to "inside" state. |
| Geofence edit on the map (move vertices) | 1 day | Leaflet editable layers or vertex handles on top of DrawLayer. |
| Antimeridian / large polygons | 1 day | `geography` type or split at ±180°; currently rejected with a clear 400. |
| Drop two unused indexes; covering index for history tie-break | 1 hour | Documented in ARCHITECTURE.md. |
| E2E browser tests in CI | 1 day | The Playwright flows I used during development → `@playwright/test` in GitHub Actions. |

### Production readiness

- **Auth & RBAC**, per-tenant data isolation (`org_id` on every table, row-level security).
- **Observability**: request IDs already in logs → ship JSON logs; Prometheus metrics (request
  latency by route, location-update rate, dispatch queue depth, WS clients, dropped clients);
  OpenTelemetry traces across the transaction and dispatch; alerts on queue depth / 5xx / DB
  pool saturation.
- **Migrations tooling**: move from the embedded runner to golang-migrate or Atlas with
  down-migrations, lint, and expand/contract rules for zero-downtime deploys.
- **HA Postgres**: managed primary + replica with automatic failover, PITR backups; read
  replicas for history and the fleet map; connection pooling (pgxpool sizing, or PgBouncer in
  session mode).
- **Delivery guarantees**: an outbox table written in the same transaction as the events, so a
  crash between commit and enqueue can't lose an alert; alert replay by `event_id` cursor on
  reconnect.
- **Security**: TLS everywhere, strict CORS (already per-origin), input size limits (done:
  1 MiB bodies, 1000 vertices), secrets in a manager, dependency scanning.

### Scaling to thousands of vehicles and geofences

- **Ingestion**: accept location batches; put updates on a durable queue (**Kafka/NATS**)
  partitioned by `vehicle_id`. Per-partition ordering replaces the row lock, consumers scale
  horizontally, and bursts are absorbed. Return 202 for batched ingest.
- **Write path**: `vehicle_locations` is append-only and grows fastest → **partition by time**
  (monthly, `pg_partman`), drop/archive old partitions; or a time-series store
  (TimescaleDB hypertables). Batch inserts with `COPY`.
- **Geofence lookups**: geofences change rarely → **cache them in memory in an R-tree**
  (e.g. `tidwall/rtree`) per instance, invalidated via `LISTEN/NOTIFY` on change; point-in-
  polygon in Go and PostGIS only for writes and history. Removes a DB round trip per update.
- **Fan-out across instances**: the hub is in-process today → **Redis pub/sub** (or NATS, or
  Postgres `LISTEN/NOTIFY`) so every backend instance broadcasts every alert to its own WS
  clients; then **scale WebSocket nodes horizontally** behind a load balancer (sticky not
  required). Subscriptions per org/zone to avoid sending every alert to everyone.
- **Backpressure** everywhere: bounded queues (already: dispatcher + per-client buffers),
  shed load with 429/503 + `Retry-After`, consumer lag alerts, per-client send limits.
- **Reads**: fleet map via a cached "current state" (Redis or a materialized view) instead of
  joining on every poll; history on replicas with keyset pagination and capped counts.
- **Numbers to quote**: one update today is a handful of indexed queries in one transaction
  (sub-ms query plans at 200k events, see EXPLAIN in ARCHITECTURE.md); the bottleneck at scale
  is DB writes per second → queue + batching + partitioning above.
