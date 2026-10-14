# Deployment

Backend: Docker Hub image → **Render** (or Fly.io) · Database: **Neon** Postgres with PostGIS ·
Frontend: **Vercel**. Replace the placeholders below:

| Placeholder | Example |
|---|---|
| `DOCKERHUB_USER` | `jane` |
| `API_HOST` | `geofence-tracker-api.onrender.com` |
| `APP_ORIGIN` | `https://geofence-tracker.vercel.app` |

Order matters only at the end: the frontend needs the backend URL at **build** time, and the
backend needs the frontend origin for CORS/WebSocket origin checks.

---

## 1. Publish the backend image (Docker Hub)

```bash
docker login                                   # once
make docker-push DOCKERHUB_USER=jane           # builds linux/amd64 + linux/arm64, pushes :v1.0.0 and :latest
```

Under the hood (see the root `Makefile`):

```bash
docker buildx create --name geofence-builder --driver docker-container   # once
docker buildx build --builder geofence-builder --platform linux/amd64,linux/arm64 \
  -t jane/geofence-tracker-backend:v1.0.0 -t jane/geofence-tracker-backend:latest --push ./backend
```

arm64 is cheap: the Dockerfile's build stage runs on the builder's native platform and
cross-compiles (`GOARCH=$TARGETARCH`, pure Go), so there is no QEMU emulation. Verify:

```bash
docker buildx imagetools inspect jane/geofence-tracker-backend:v1.0.0   # lists linux/amd64 and linux/arm64
```

The image (distroless, non-root, ~28 MB) also contains `/app/seed` and `/app/simulator`.

## 2. Database: Neon (managed Postgres with PostGIS)

1. Create a project at <https://neon.tech> in the region closest to the backend
   (e.g. AWS us-west-2 for Render Oregon).
2. **Connection details → turn "Connection pooling" OFF** and copy the *direct* string
   (host without `-pooler`). Keep `?sslmode=require`:
   `postgres://user:pass@ep-xxx.us-west-2.aws.neon.tech/neondb?sslmode=require`
   - Why direct: pgx caches prepared statements and the migration runner takes a
     session-level advisory lock; a transaction-mode pooler (PgBouncer) breaks both.
3. Nothing else to do: on first start the backend runs the embedded migrations, and
   `0001_init.sql` begins with `CREATE EXTENSION IF NOT EXISTS postgis;` (Neon permits this
   for the project owner role). Check after the first deploy:

   ```bash
   psql "$DATABASE_URL" -c "SELECT postgis_version();"
   psql "$DATABASE_URL" -c "SELECT version FROM schema_migrations ORDER BY 1;"   # 0001…0003
   ```

**Alternatives**

- **Supabase**: Database → Extensions → enable `postgis` (or let the migration do it). Use the
  *session* connection string (port 5432), not the transaction pooler (6543). If PostGIS was
  enabled into the `extensions` schema, make sure it is on the role's `search_path`
  (Supabase's default `postgres` role includes it).
- **Render Postgres**: supports PostGIS; create it in the same region and use its *Internal*
  URL for `DATABASE_URL`. Check the current free-tier expiry before relying on it.

## 3a. Backend on Render (recommended)

1. Edit `render.yaml`: set the image to `docker.io/jane/geofence-tracker-backend:v1.0.0` and
   `ALLOWED_ORIGINS` to your Vercel origin (you can fix it after step 4).
2. Render Dashboard → **New → Blueprint** → select this repository → apply.
3. When prompted, set `DATABASE_URL` to the Neon string from step 2.
4. Wait for the health check (`/health`) to go green; your URL is `https://API_HOST`.

```bash
curl https://API_HOST/health          # {"status":"ok","db":"ok","time_ns":"…"}
```

Notes:
- Render sets `PORT`; the server reads it. TLS terminates at Render, so clients use
  `https://` and `wss://`.
- WebSockets are supported. Free instances **spin down when idle**: the first request after
  that is a cold start and open sockets are dropped (clients reconnect with backoff). Use a
  paid instance (`plan: starter`) for the demo/grading window.
- Keep **one instance** (`numInstances: 1`): the WebSocket hub is in-process.

## 3b. Backend on Fly.io (alternative)

```bash
fly auth login
# edit fly.toml: image = docker.io/jane/…, ALLOWED_ORIGINS, a unique app name
fly launch --no-deploy --copy-config
fly secrets set DATABASE_URL='postgres://…?sslmode=require'
fly deploy
fly scale count 1
curl https://<app>.fly.dev/health
```

`auto_stop_machines = "off"` and `min_machines_running = 1` keep the single machine (and its
WebSocket clients) alive.

### WebSocket idle timeouts

Proxies close connections that carry no traffic for a while. Ours never go quiet for long:
the server sends a WebSocket **ping every 30 s** (clients answer automatically) and the
frontend sends an application `{"type":"ping"}` **every 25 s** (server answers
`{"type":"pong"}`; the client reconnects if no reply within 10 s). Any proxy idle timeout
above ~30 s is therefore never reached; both Render and Fly allow longer idle periods than
that, but check your plan's current limits if you change providers.

## 4. Frontend on Vercel

1. Vercel → **Add New → Project** → import the repo → **Root Directory: `frontend`**.
   `frontend/vercel.json` sets the build (`npm ci`, `npm run build`, output `dist`), SPA
   rewrites (every path → `index.html`, so deep links like `/history?page=2` work) and caching.
2. **Environment Variables** (Production; these are inlined at *build* time, so redeploy
   after changing them):

   | Name | Value |
   |---|---|
   | `VITE_API_URL` | `https://API_HOST` |
   | `VITE_WS_URL` | `wss://API_HOST/ws/alerts` |

   `VITE_WS_URL` is optional (derived from `VITE_API_URL`: https → wss) but explicit is clearer.
3. Deploy. Your origin is `APP_ORIGIN`.

> **Mixed content:** a page served over `https://` may not call `http://` or open `ws://`;
> browsers block it silently. With an https frontend, the API must be `https://` and the
> socket `wss://`. The header's connection pill stays red ("Offline") if this is wrong.

## 5. Allow the frontend origin on the backend

`ALLOWED_ORIGINS` controls both CORS and the WebSocket origin check. Exact origins,
comma-separated, scheme included, no trailing slash:

```text
ALLOWED_ORIGINS=https://geofence-tracker.vercel.app
```

- Render: Service → Environment → edit → save (redeploys). Fly: edit `fly.toml` `[env]` and
  `fly deploy`.
- Vercel *preview* deployments get different hostnames; add them to the list if you use
  them. `*` works but disables the origin protection.
- Check: `curl -si -H "Origin: APP_ORIGIN" https://API_HOST/geofences | grep -i access-control-allow-origin`

## 6. After deploying

```bash
# API contract (creates uniquely named test data; deletes its alert rules afterwards)
make verify BASE_URL=https://API_HOST
# Demo data (idempotent), using the published image, no Go needed
make seed-remote DOCKERHUB_USER=jane BASE_URL=https://API_HOST
# …or with Go: cd backend && go run ./cmd/seed -base-url https://API_HOST
# Drive the demo truck through the restricted zone
docker run --rm --entrypoint /app/simulator jane/geofence-tracker-backend:v1.0.0 \
  -base-url https://API_HOST -vehicle CA-TRK-1001
# Watch alerts
websocat wss://API_HOST/ws/alerts            # or: npx wscat -c wss://API_HOST/ws/alerts
```

Then open `APP_ORIGIN`: the header should show **Live**, and the simulator's restricted-zone
entry should toast within a second.

### Troubleshooting

| Symptom | Likely cause |
|---|---|
| Frontend loads, every call fails, console shows CORS error | `ALLOWED_ORIGINS` doesn't exactly match `APP_ORIGIN` |
| Header stays "Offline"/"Reconnecting" | `VITE_WS_URL` is `ws://` on an https page, or origin not allowed (403 on upgrade) |
| Backend crash-loops at start | `DATABASE_URL` wrong / pooled; check logs for `migration` or `connect` |
| `prepared statement … already exists` errors | Pooled (`-pooler`) connection string; use the direct one |
| First request slow, sockets drop | Free instance spun down; use a paid plan for the demo |
