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
| **Live frontend** | <https://YOUR-APP.vercel.app> *(placeholder)* |
| **Backend API** | <https://YOUR-API.onrender.com> *(placeholder)*, e.g. `/health`, `/geofences` |
| **Docker Hub image** | [`DOCKERHUB_USER/geofence-tracker-backend:v1.0.0`](https://hub.docker.com/r/DOCKERHUB_USER/geofence-tracker-backend) *(placeholder)* |
| **Video walkthrough** | *(link to be added)* |

![Dashboard: fleet map with geofences, live alerts panel](docs/images/dashboard.png)
*(screenshot placeholder: add `docs/images/dashboard.png`)*

## Quick start

```bash
make up        # PostGIS + backend + frontend via Docker Compose → http://localhost:4173
make seed      # San Francisco demo data (geofences, vehicles, alert rules)
make simulate  # drive a truck through the restricted zone and watch the alerts
```

Everything else (prerequisites, running without Docker, environment variables, a curl example
for every endpoint, WebSocket testing and a frontend guide) is in **[SETUP.md](SETUP.md)**.

## Documentation

- [SETUP.md](SETUP.md): setup, API testing guide, frontend usage
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): components, location-update sequence, schema and
  index reasoning, design decisions, measured query plans, edge cases, known limitations
- [docs/EXTRA_ENDPOINTS.md](docs/EXTRA_ENDPOINTS.md): additions beyond the spec (`/health`,
  `/fleet/locations`, `PATCH`/`DELETE /alerts/{id}`, `/alerts/notifications`, WebSocket protocol)
- [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md): Docker Hub, Render/Fly.io, Neon PostGIS, Vercel
- [docs/ASSIGNMENT.md](docs/ASSIGNMENT.md): the original brief

## Highlights

- All nine specified endpoints with the exact paths, fields and `time_ns` on every response,
  including errors; `scripts/verify_contract.sh` checks the whole contract (37 checks) and the
  WebSocket alert shape against any deployment.
- Correct under concurrency: per-vehicle row locking, tested with 20 parallel updates.
- Boundary points count as inside; overlapping/nested zones; out-of-order and duplicate reports
  handled; database outages return 503 and recover without a restart.
- Real-time: one WebSocket per tab with reconnect/backoff/heartbeat, history backfill after
  reconnects, toasts and map pulses; alert dispatch never blocks location updates.
- Tests: Go unit + PostGIS integration tests (race detector clean), Vitest for the frontend's
  validation and socket logic.
