# Extra endpoints

Endpoints beyond the nine specified in [ASSIGNMENT.md](ASSIGNMENT.md). The
specified endpoints are unchanged; these are additive. Every response still
includes `time_ns` and uses the standard error shape.

## GET /health

Liveness + database check. `200 {"status":"ok","db":"ok","time_ns":"..."}`,
or `503` with `"status":"degraded","db":"unavailable"`.

## GET /fleet/locations

Every vehicle's last known location and the geofences it is currently
inside, in **one query**. The frontend map uses this instead of calling
`GET /vehicles/location/{vehicle_id}` once per vehicle (N+1 requests).

```bash
curl -s localhost:8080/fleet/locations
```

```json
{
  "vehicles": [
    {
      "vehicle_id": "veh_456",
      "vehicle_number": "KA-01-AB-1234",
      "driver_name": "John Doe",
      "vehicle_type": "truck",
      "status": "active",
      "current_location": {
        "latitude": 37.7849,
        "longitude": -122.4194,
        "timestamp": "2025-01-15T10:35:00Z"
      },
      "current_geofence_ids": ["geo_123"]
    },
    {
      "vehicle_id": "veh_789",
      "vehicle_number": "KA-02-CD-5678",
      "driver_name": "Jane Roe",
      "vehicle_type": "van",
      "status": "active",
      "current_location": null,
      "current_geofence_ids": []
    }
  ],
  "time_ns": "812345"
}
```

- `current_location` is `null` until the vehicle's first `POST /vehicles/location`.
- `current_geofence_ids` is always an array (sorted); resolve names via `GET /geofences`.
- Vehicles are ordered newest first, like `GET /vehicles`.

## PATCH /alerts/{alert_id}

Activate or deactivate a rule ("manage existing alerts"). Body:
`{"status": "active" | "inactive"}`. Returns `200` with the full rule (same
shape as an item of `GET /alerts`) plus `time_ns`. `404` unknown id, `400`
invalid status, `409` if reactivating would duplicate another active rule.

```bash
curl -s -X PATCH localhost:8080/alerts/alert_789 -H 'Content-Type: application/json' -d '{"status":"inactive"}'
```

## DELETE /alerts/{alert_id}

Deletes a rule. Returns `200 {"alert_id":"alert_789","deleted":true,"time_ns":"..."}`
(200 with a body rather than 204, because every response carries `time_ns`).
`404` if the rule does not exist. Past alert notifications are kept.

## Alert rule conventions (apply to the specified endpoints too)

- **All-vehicle rules**: when `vehicle_id` is omitted in `POST /alerts/configure`,
  the response has `"vehicle_id": null`, and in `GET /alerts` both `vehicle_id`
  and `vehicle_number` are `null`. The frontend renders this as "All vehicles".
- **Duplicates**: an identical *active* rule (same geofence, same vehicle or
  both "all", same event_type) returns `409` with
  `"details": {"existing_alert_id": "..."}`. Inactive rules don't count.
- **`GET /alerts?vehicle_id=`** is an exact filter: it does not include
  all-vehicle rules.

## GET /alerts/notifications?limit=

Recently dispatched alerts, newest first, so the live feed can show history
after a page reload. `limit` defaults to 50 (1–200, else `400`). Each item is
exactly the `/ws/alerts` message that was broadcast, in the same field order.

```json
{"notifications":[{"event_id":"evt_ql6zewccjuh6","event_type":"entry","timestamp":"2026-10-13T06:31:00Z",
  "vehicle":{"vehicle_id":"veh_h4coqaevly3n","vehicle_number":"KA-01-AB-1234","driver_name":"John Doe"},
  "geofence":{"geofence_id":"geo_qbn5eymnvpwh","geofence_name":"Airport No-Go","category":"restricted_zone"},
  "location":{"latitude":37.62,"longitude":-122.38}}],"time_ns":"1878623"}
```

## WebSocket protocol (`/ws/alerts`)

- **Alerts** are sent as the bare object specified in ASSIGNMENT.md (no envelope).
- **Control messages** carry a `type` field: on connect the server sends
  `{"type":"connected","client_id":"ws_…","server_time":"…","message":"…"}`;
  a client may send `{"type":"ping"}` and receives `{"type":"pong"}`. Other
  inbound messages are ignored. Clients distinguish: `"type" in msg` → control,
  `"event_id" in msg` → alert.
- The server also sends protocol-level pings every 30s; idle clients that stop
  answering are disconnected after 60s. Browsers' Origin must be in
  `ALLOWED_ORIGINS` (`403` JSON otherwise); clients without an Origin header
  (CLI tools) are allowed.
- Alerts are created once per (event, alert rule): two active rules matching the
  same event produce two alerts with different `event_id`s.

Watch alerts from a terminal:

```bash
npx wscat -c ws://localhost:8080/ws/alerts      # or: websocat ws://localhost:8080/ws/alerts
```
