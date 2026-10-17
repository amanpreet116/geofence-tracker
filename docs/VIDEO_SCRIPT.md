# Video script (private, not graded)

Target: **35–40 min** · Demo 10–12 · Code 13–16 · Gaps & roadmap 10–12.
`[DO]` = action on screen · `[SHOW]` = what's visible · plain text = what to say.
Speak it in your own words; the graders score *understanding*, so treat the lines as a guide,
not something to read verbatim. ~130 words/min → each section's text fits its time.

---

## Before you hit record (checklist)

- [ ] `make clean && make up && make seed` (fresh local stack: http://localhost:4173, API :8080)
- [ ] Open the **deployed** frontend once to wake it; check the header says **Live**
- [ ] Production test data removed (the "Contract Restricted …" geofences / `CT-…` vehicles), otherwise the map zooms out
- [ ] Terminal A: repo root, large font. Terminal B: ready with `npx wscat -c ws://localhost:8080/ws/alerts`
- [ ] Editor open at the repo root, files closed, sidebar visible; browser zoom 110%
- [ ] Notifications off; close unrelated tabs
- [ ] Have `docs/ARCHITECTURE.md` open in a preview tab (diagrams)

---

## 0. Opening (≈1 min)

[SHOW] README.md on GitHub or in the editor.

> Hi, I'm Amanpreet. This is my submission for the full-stack geofencing and real-time alert
> assessment. In the next forty minutes I'll do three things: first, a live demo of the
> complete system, from drawing a geofence to receiving a real-time alert; second, a walk
> through the code: the Go backend, the PostGIS queries, the WebSocket pipeline and the React
> frontend; and third, an honest look at what's incomplete, what I'd improve, and how I'd scale
> it to thousands of vehicles.
>
> Quick summary of the stack: the backend is Go using only the standard library's HTTP router,
> PostgreSQL 16 with PostGIS for the geospatial work, and gorilla/websocket for real-time
> alerts. The frontend is React with TypeScript, Vite, Tailwind, react-leaflet for maps and
> TanStack Query for server state. Everything runs with Docker Compose locally, and it's
> deployed: the backend image is on Docker Hub, running on Railway with a Supabase Postgres
> database, and the frontend is on Vercel.

---

## 1. App demo (10–12 min)

### 1.1 Docker setup (1 min)

[DO] Terminal A: `make ps` (stack already up). Optionally show `docker-compose.yml`.

> Everything starts with one command, `make up`, which is Docker Compose with three services:
> PostGIS, the Go backend and the frontend served by nginx. All three report healthy. The
> backend image is distroless, runs as a non-root user and is about 28 megabytes. Because it
> has no shell, I built a `healthcheck` subcommand into the server binary itself for the
> container health check. Database migrations are embedded in the binary and run
> automatically at startup.

[DO] `make seed` (run it twice).

> `make seed` loads a San Francisco demo dataset *through the public API*, so it doubles as a
> smoke test. Five geofences covering all four categories, four vehicles and four alert rules.
> If I run it again, every line shows an equals sign: it's idempotent. Geofences are matched by
> name, and vehicles and rules rely on the API's 409 conflict responses.

### 1.2 Dashboard (1 min)

[DO] Browser → http://localhost:4173. [SHOW] dashboard.

> This is the dashboard. In the header there's the live-connection indicator. It's green and
> says "Live", meaning the browser has an open WebSocket to the backend. Next to it is an
> alerts bell with recent alerts, and a theme switcher.
>
> The map shows every geofence colored by category: red is restricted, blue is delivery, amber
> is toll, green is customer area. Vehicles appear as markers at their last reported position,
> and vehicles that have never reported get a hint at the bottom instead of a marker.

### 1.3 Create a geofence by drawing (2 min)

[DO] Geofences → **New geofence** → name "Pier 39 Restricted", category Restricted zone →
click 5 points around Pier 39 → **Undo point** once → click the first vertex.

> Creating a geofence. I give it a name and category, and draw it by clicking the map. The
> preview updates as I go. Undo removes the last point. I close the polygon by clicking the
> first vertex, or with Finish, and the app appends the first point again, because the API
> requires a closed ring: the first and last coordinates must be identical.

[DO] Switch to **Enter coordinates**.

> The same polygon as text. The two input modes share one list of points, so drawing fills
> this box and typing here, then pressing Preview, redraws the map. It accepts JSON pairs or
> one "lat, lng" per line.

[DO] Paste the bowtie `[[37.80,-122.44],[37.81,-122.43],[37.81,-122.44],[37.80,-122.43],[37.80,-122.44]]`
→ Preview → Create.

> Validation happens in two layers. The client mirrors the backend rules: at least four
> points, closed ring, latitude and longitude ranges, a thousand-vertex cap. That's the shape
> you saw. But this one is a bowtie: it crosses itself. It passes every simple rule, so it
> reaches the server, where PostGIS's `ST_IsValid` rejects it, and the error appears right
> under the field with the intersection point in latitude-longitude order.

[DO] Go back to the drawn polygon → **Create geofence** → toast; list selects it, map flies to it.

> With the valid polygon: created, a success toast, and the list selects it and flies to it.

[DO] Category filter → Restricted zone; point at the URL `?category=restricted_zone`; hover/click items.

> The list filters by category. The filter is in the URL and goes to the API as the
> `category` query parameter, so the view is shareable. Hovering highlights a polygon and
> clicking zooms to it.

### 1.4 Register a vehicle and update its location (2 min)

[DO] Vehicles → **Register vehicle** → number `ca-dem-0001`, driver, type, phone. Point at the
"Will be saved as CA-DEM-0001" hint → Register. Then register the same number again.

> Registering a vehicle. The server normalizes vehicle numbers, uppercasing them and collapsing
> spaces, and the form tells you up front what will be stored. Registering the same number
> again gives a 409 conflict, shown on the field. The uniqueness is enforced by a database
> constraint, not by a check-then-insert, so two simultaneous requests can't both succeed.

[DO] In *Update location*: select the vehicle → click the map inside Downtown → **Update location**.

> To move a vehicle, I select it and click the map, or type coordinates. The timestamp defaults
> to now and is sent as UTC. When I submit, the marker moves immediately. That's an optimistic
> update; it's reconciled with the server's response, and if the server ignored the report,
> for example because it's older than the last one, the marker snaps back. The result shows
> the geofences the vehicle is now inside, and a toast says it entered Downtown Delivery Zone.

[DO] **Details** on the vehicle row.

> The details drawer is the `GET /vehicles/location/{id}` endpoint: current location and
> current geofences with their categories.

### 1.5 Configure alerts (1 min)

[DO] Alerts → geofence *Pier 39 Restricted*, vehicle **All vehicles**, trigger **Entry & exit** →
Create. Click Create again. Toggle a rule's switch off and on.

> Alert rules: a geofence, a vehicle or "All vehicles", and the trigger: entry, exit, or
> both. When the vehicle is omitted, the API stores it as null and returns null, and the UI
> shows "All vehicles". Creating an identical active rule again gives a 409. That comes from a
> partial unique index over *active* rules only, so a paused rule doesn't block creating a new
> one. Rules can be paused and deleted; those two endpoints are additions I documented
> separately from the spec.

### 1.6 Real-time alerts (2–3 min) ⭐

[DO] Terminal B: `npx wscat -c ws://localhost:8080/ws/alerts` → show the `connected` message.

> Here's a raw WebSocket client on `/ws/alerts`. The server sends a hello message, and then
> alerts as bare JSON objects in exactly the format from the assignment.

[DO] Terminal A: `make simulate VEHICLE=CA-TRK-1001,CA-VAN-2002 SPEED=400`. Switch to the
Dashboard → Live alerts tab.

> Now the simulator: a small Go program, also inside the Docker image, that drives two vehicles
> along a route through Mission, Downtown, SoMa and into the Port Terminal Restricted Area,
> posting to `POST /vehicles/location` every second. Lines marked with a star are where a
> vehicle's set of geofences changed.
>
> Watch the browser: as the truck enters the restricted zone, a red toast appears. Red because
> it's a restricted-zone entry. It shows the vehicle number, driver, zone, category, time and
> coordinates. The map pulses where it happened, the Live alerts tab gets a badge, and the
> raw client in the other terminal received the same JSON. The van's exit from the Mission
> customer area also alerts, because of the van-specific exit rule.
>
> Clicking an alert flies the map to its location.

### 1.7 History (1–2 min)

[DO] History → vehicle CA-TRK-1001 → per page 25 → page 2 → copy URL → open in new tab.
Click a row. Export page (CSV). Set *From* after *To*.

> Violation history is the entry and exit log. Filters for vehicle, geofence and a date-time
> range, plus page size. Pagination is server-side, with limit and offset, and the footer
> shows "Showing 26–50 of N" using the API's total count. Every filter lives in the URL: if I
> open this link in a new tab, I get exactly the same view. Clicking a row shows the point and
> the geofence on a mini map, and I can export the current page as CSV. An invalid range,
> "From" after "To", is caught inline.

### 1.8 Resilience and deployment (1 min)

[DO] `docker compose stop backend` → watch the pill go amber, then red → `docker compose start backend`
→ back to Live.

> One more thing: I stop the backend. The indicator goes to "Reconnecting", then "Offline".
> The client retries with exponential backoff and jitter. When the backend is back, it
> reconnects, and alerts that fired in between are backfilled from the history endpoint.

[DO] Terminal A: `make verify` (local) — show "37 passed, 0 failed". Then open the deployed URLs.

> Finally, I wrote a contract-verification script in bash, curl and jq. It checks all
> nine endpoints: exact paths, field types, `time_ns` as a numeric string on every response
> including errors, the 400, 404 and 409 paths, and an end-to-end scenario with a WebSocket
> listener. 37 checks, all passing, and I ran the same script against the production
> deployment on Railway.

---

## 2. Implementation deep dive (13–16 min)

### 2.1 Architecture overview (1.5 min)

[SHOW] docs/ARCHITECTURE.md → component diagram.

> The repository is a monorepo: `backend`, `frontend`, `docs`. The backend is layered:
> handlers only decode and encode HTTP; services hold the business logic; repositories hold
> hand-written SQL. There's no ORM. Off to the side are the alert dispatcher and the
> WebSocket hub. Services declare the interfaces they need and repositories implement them,
> so the core logic is testable without a database.

[SHOW] the sequence diagram.

> And this is the most important flow: a location update. Everything that has to be
> consistent happens in one database transaction; alert delivery happens after commit and
> never delays the HTTP response. I'll show each piece in code.

### 2.2 `cmd/server/main.go` (1 min)

[DO] Open it; scroll through `run()`.

> `main` is wiring only, no global state. It loads config from environment variables and
> fails fast with clear messages. It connects to Postgres with retry and backoff, because in
> Docker the database may not be ready yet. Then it runs migrations, starts the hub and the
> dispatcher, and builds the router.
>
> The middleware order matters: timing is outermost, so even a panic or a CORS preflight
> gets a start time for `time_ns`. Then request IDs, access logging, panic recovery and CORS.
>
> Shutdown is ordered: stop accepting HTTP, drain the alert queue, which still needs the
> database and the hub, then close WebSocket clients, then the database pool.

### 2.3 The API contract: `handler/respond/respond.go` (1.5 min)

[DO] Open it; show `Timed`, `JSON`, `classify`.

> The assignment says every response must include `time_ns` as a string. I enforce that
> structurally: every response struct embeds this `Timed` type as its last field, and the
> `JSON` helper stamps the elapsed nanoseconds just before writing. If I forget to embed it,
> the code doesn't compile. I deliberately didn't marshal into a map, because Go sorts map
> keys alphabetically, which would scramble the field order compared with the spec.
>
> `classify` is the one place errors become status codes: validation errors become 400 with
> per-field details, not-found 404, conflicts 409, and a database outage becomes 503 with a
> Retry-After header, not a 500. Unknown errors become a generic 500 and the real error is only
> logged, never sent to the client. Unknown routes and wrong methods also return JSON with
> `time_ns`; the router wraps Go's ServeMux to replace its plain-text 404 and 405.

### 2.4 Coordinates: `repository/geo.go` (1 min)

[DO] Open it; point at the header comment, `splitRing`, `pointSQL`.

> A classic source of bugs: the API speaks latitude-longitude, but PostGIS and WKT are
> longitude-latitude, x then y. This file is the only place in the codebase that converts
> between them. Coordinates are sent to Postgres as binary float arrays, not as text, so they
> round-trip exactly, and I have a test that stores awkward floating-point values and checks
> they come back bit-for-bit identical.

### 2.5 The core: `service/location.go` (3 min)

[DO] Open `UpdateLocation`; walk through the transaction.

> This is the heart of the system. Inside one transaction:
>
> First, `LockVehicle` does `SELECT … FOR UPDATE` on the vehicle row. Why? Entry and exit are
> computed by diffing the stored "inside" set against the new position. If two updates for the
> same vehicle ran concurrently, both could read the same "before" state and both emit an
> entry. The row lock serializes updates *per vehicle*; different vehicles never wait for each
> other.
>
> Then two special cases. An exact duplicate of the last report, same timestamp and position,
> is a no-op, so client retries are safe. And a report *older* than the last applied one is
> stored in history but doesn't change state or produce events. Applying it would "rewind" the
> vehicle and invent exit-entry pairs that never happened in real time.
>
> Otherwise: insert the location, find the containing geofences, and call `ComputeTransitions`.
> That's a pure function: previous set, current set, returns entered and exited. Because it's
> pure it's unit-tested without a database, including nested and overlapping zones.
>
> Then the events are inserted as violations with `viol_` IDs, the state table is updated,
> and we commit. Only *after* commit do we call `Dispatch`, which just enqueues. The response
> never waits for alerts.

### 2.6 The geospatial query: `repository/location.go` (1.5 min)

[DO] Open `ContainingGeofences`; show the comment.

> The containment query uses `ST_Covers`, not `ST_Contains`. `ST_Contains` is false for a point
> exactly *on* the boundary. A vehicle on the fence line of a restricted zone should alert, and a
> vehicle driving along an edge shouldn't flap between entry and exit. `ST_Covers` treats the
> boundary as inside and is just as fast, because it uses the GiST index on the boundary
> column to pre-filter by bounding box. There's an integration test with points on an edge and
> on a vertex.

### 2.7 Proving the concurrency: `service/location_integration_test.go` (1.5 min)

[DO] Open `TestConcurrentUpdates`.

> To prove the locking works, this test fires 20 location updates for one vehicle at the same
> time, alternating inside and outside. Arrival order is random, so some become out-of-order.
> The test replays the updates that were actually applied, in timestamp order, computes which
> entries and exits *should* exist, and requires the database to contain exactly those, never
> two entries in a row. I verified the test is meaningful: I temporarily removed `FOR UPDATE`
> and it failed immediately with duplicate entries; with the lock it passes reliably under the
> race detector.

### 2.8 Alert pipeline: `service/alert_dispatcher.go`, `service/alert.go` (2 min)

[DO] Open the dispatcher: `Dispatch`, `Start`, `process`, `notify`.

> `Dispatch` is a non-blocking channel send onto a queue of 1,024 batches, processed by a
> fixed pool of four workers. Not a goroutine per request, which would be unbounded under
> load. If the queue is ever full, I log a warning and process inline. That's backpressure,
> chosen over silently losing alerts for events that are already committed.
>
> A worker fetches candidate rules for the whole batch in one SQL query, then the pure
> function `RuleMatches` makes the exact decision: same geofence, the rule is for all vehicles
> or this vehicle, the event type matches or the rule is "both", and the rule is active.
>
> Each match is stored in `alert_notifications` with `ON CONFLICT DO NOTHING` against a
> unique index on event and rule, so dispatch is idempotent: one notification per event and
> rule, even if a batch is processed twice. Only a *newly inserted* notification is broadcast.

### 2.9 WebSocket hub: `realtime/hub.go`, `client.go` (1.5 min)

[DO] Open `Run`, then `writePump`/`readPump`.

> The hub follows the classic pattern: one goroutine owns the set of clients, and register,
> unregister and broadcast are channel messages, so there are no mutexes. Each client has a
> 256-message buffer; if a client stops reading and its buffer fills, the hub drops *that*
> client instead of blocking everyone else.
>
> Each connection has a write loop, the only goroutine that writes to the socket, which sends
> messages and a ping every 30 seconds, and a read loop that extends the deadline on every
> pong. The origin check follows `ALLOWED_ORIGINS`. Alerts go out as the bare spec object, no
> envelope; control messages have a `type` field, which is how clients tell them apart.

### 2.10 Database schema: `db/migrations/0001_init.sql` (1 min)

[DO] Open the migration; [SHOW] the index table in ARCHITECTURE.md.

> The schema: geofences with a PostGIS polygon and a validity check, vehicles, an append-only
> location history, a small table of which geofences each vehicle is currently inside, alert
> rules, events and notifications. Every index has a reason. The GiST index serves the
> containment query on every update. The history indexes, vehicle-time, geofence-time and
> time, make the history endpoint a top-N index scan. I measured it with EXPLAIN ANALYZE on
> 200,000 synthetic events: about 0.3 milliseconds for a filtered page. I also documented two
> indexes that turned out to be redundant or unused.

### 2.11 Frontend structure (2 min)

[DO] Open `frontend/src/api/types.ts`, `api/hooks.ts`, then `realtime/alertSocket.ts`.

> On the frontend, `api/types.ts` mirrors the backend JSON exactly, in snake_case, including
> `time_ns`. All server state goes through TanStack Query hooks in `hooks.ts`; mutations
> invalidate the right queries, and the location update is optimistic with rollback.
>
> The real-time layer is a framework-free class, `AlertSocket`, wrapped by a React provider.
> It reconnects with exponential backoff and full jitter, capped at 30 seconds, retries
> immediately when the window regains focus or the network comes back, and sends an
> application ping every 25 seconds to detect dead connections. Because it's a plain class,
> it's unit-tested with fake timers and a fake WebSocket. One subtle point: React StrictMode
> mounts components twice in development, which normally opens two sockets; the connect is
> deferred by a tick so the first mount is cancelled before it connects.
>
> The provider seeds the alert feed from the notifications endpoint, so history survives a
> reload, and dedupes by `event_id`.

[DO] Briefly open `components/map/DrawLayer.tsx`.

> Drawing uses react-leaflet's `useMapEvents` instead of an unmaintained draw plugin. And the
> client-side validation uses the same error keys as the server's `details`, so server errors
> land next to the same fields without any mapping.

---

## 3. Gaps, improvements & roadmap (10–12 min)

### 3.1 What's incomplete (3 min)

> I'll be upfront about what's not done.
>
> **Rate limiting and authentication**, both bonus items, are not implemented. The
> configuration exists, `RATE_LIMIT_RPS`, `API_KEYS`, `AUTH_ENABLED`, and is validated at
> startup, the 429 and 401 responses are already mapped in the error handler, and the
> frontend already sends an `X-API-Key` header if one is configured. What's missing is the
> middleware itself: a token bucket per key or IP using `x/time/rate`, about half a day, and
> API-key auth, another half day. JWT with roles would be two to three days.
>
> **Pagination** exists on violation history and the notifications feed, but not on the other
> list endpoints. I'd use cursor pagination on created_at and ID: about half a day.
>
> **Editing and deleting** geofences and vehicles: there's no API for it because the spec
> doesn't have one. The database cascades are in place. About a day including the UI.
>
> *(If the redesign isn't finished:)* **The frontend visual redesign** is partially complete:
> the design tokens, light and dark themes and the new app shell are done, with contrast
> checked against WCAG AA, but the individual pages still use the earlier styling. One to two
> days to finish.
>
> **End-to-end browser tests**: I verified every flow with Playwright scripts during
> development, but they're not in the repo or in CI yet. About a day.

### 3.2 Edge cases: handled and known limits (2 min)

> Edge cases I handled and tested: a point exactly on the boundary counts as inside;
> overlapping and nested geofences; a vehicle reporting before any geofence exists;
> out-of-order and duplicate reports; concurrent updates; a rule deleted while its alert is
> being dispatched; a WebSocket client that never reads; and the database going down at
> runtime, where the health check reports it, the API returns 503, and it recovers without a
> restart.
>
> Known limits: geometry is planar in degrees, which is exact for city-scale zones but can't
> represent a polygon crossing the antimeridian. Rather than silently storing it wrapped the
> wrong way round the globe, I reject it with a clear message. The fix is the `geography`
> type or splitting at 180 degrees. The WebSocket hub is in-process, so the backend must run
> as a single instance. And overlapping rules, say an entry rule and a both rule for the same
> zone, produce two alerts for one event.

### 3.3 Production readiness (2 min)

> To make this production-ready, beyond auth and rate limiting:
>
> - **Observability**: request IDs are already in every log line; next would be Prometheus
>   metrics: latency per route, location updates per second, dispatch queue depth, WebSocket
>   clients and dropped clients, plus OpenTelemetry traces across the transaction and the
>   dispatcher.
> - **Delivery guarantees**: today, if the process crashed between commit and enqueue, an
>   alert could be lost. The fix is a transactional outbox, written in the same transaction as
>   the events and drained by the dispatcher.
> - **Migrations**: move from my embedded runner to golang-migrate or Atlas, with down
>   migrations and expand-contract changes for zero-downtime deploys.
> - **Database**: a managed primary with a replica and point-in-time recovery; read replicas for
>   history and the fleet map.
> - Multi-tenancy with an organization ID on every table, and row-level security.

### 3.4 Scaling to thousands of vehicles and geofences (3–4 min)

> Finally, scaling. Today a location update is a handful of indexed queries in one
> transaction, which is plenty for hundreds of vehicles. At thousands of vehicles reporting
> every few seconds, the bottleneck becomes database writes per second.
>
> **Ingestion**: put location updates on a durable queue like Kafka or NATS, partitioned by
> vehicle ID. Per-partition ordering gives the same guarantee the row lock gives today, so
> consumers can scale horizontally and absorb bursts, with batched inserts using COPY.
>
> **Storage**: the location history table is append-only and grows fastest, so I'd partition
> it by time, monthly, and drop or archive old partitions, or use TimescaleDB.
>
> **Geofence lookups**: geofences change rarely, so each instance can cache them in memory in
> an R-tree and do the point-in-polygon check in Go, invalidated via Postgres LISTEN/NOTIFY.
> That removes a database round trip from every update.
>
> **Real-time fan-out**: with more than one backend instance, each one only knows its own
> WebSocket clients, so I'd publish alerts through Redis pub/sub or NATS and have every
> instance broadcast to its clients. Then the WebSocket tier scales horizontally behind a
> load balancer, with subscriptions per organization or zone so clients only receive
> what's relevant.
>
> **Backpressure** everywhere: the bounded queues and per-client buffers I already have,
> plus 429 or 503 with Retry-After when overloaded, and alerts on consumer lag.

---

## 4. Closing (30 s)

> To sum up: all nine endpoints match the contract exactly, verified by an automated script
> against both local and production; geofence detection is done in PostGIS with correct
> boundary handling and per-vehicle locking; alerts are delivered in real time without
> blocking location updates; and the frontend covers every required feature. The setup,
> API guide, architecture and deployment notes are all in the repository. Thank you for
> watching.

---

## Timing cheat-sheet

| Section | Target | Cumulative |
|---|---|---|
| 0 Opening | 1:00 | 1:00 |
| 1 Demo | 11:00 | 12:00 |
| 2 Deep dive | 15:00 | 27:00 |
| 3 Gaps & roadmap | 11:00 | 38:00 |
| 4 Closing | 0:30 | 38:30 |

If you run long, shorten 2.10 and 2.11; if short, demo the History CSV and the light theme.
