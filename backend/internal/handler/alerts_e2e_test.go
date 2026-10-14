package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/amanpreet116/geofence-tracker/backend/internal/middleware"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/realtime"
	"github.com/amanpreet116/geofence-tracker/backend/internal/repository"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

// slowMatcher delays rule matching so the test can prove the HTTP response
// does not wait for alert dispatch.
type slowMatcher struct {
	next  service.AlertMatcher
	delay time.Duration
}

func (m slowMatcher) Match(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertMatch, error) {
	time.Sleep(m.delay)
	return m.next.Match(ctx, events)
}

// TestAlertEndToEnd: real HTTP + WebSocket servers, middleware, dispatcher
// and PostGIS. A location update that enters a geofence with a configured
// rule must return immediately and push the spec-shaped alert within 2s.
func TestAlertEndToEnd(t *testing.T) {
	pool := testutil.Pool(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := context.Background()

	hub := realtime.NewHub([]string{"*"}, logger)
	hubCtx, stopHub := context.WithCancel(ctx)
	t.Cleanup(stopHub)
	go hub.Run(hubCtx)

	const matchDelay = 500 * time.Millisecond
	notifications := repository.NewNotificationRepository(pool)
	alertSvc := service.NewAlertService(repository.NewAlertRepository(pool), notifications)
	dispatcher := service.NewAlertDispatcher(slowMatcher{alertSvc, matchDelay}, notifications, hub, service.DispatcherConfig{Workers: 2}, logger)
	dispatcher.Start()
	t.Cleanup(func() { _ = dispatcher.Shutdown(ctx) })

	router := testRouter(Handlers{
		Locations: NewLocationHandler(service.NewLocationService(repository.NewLocationRepository(pool), dispatcher)),
		Alerts:    NewAlertHandler(alertSvc),
		WS:        http.HandlerFunc(hub.ServeWS),
	})
	srv := httptest.NewServer(middleware.Chain(router,
		middleware.Timing, middleware.RequestID, middleware.AccessLog(logger), middleware.Recover(logger), middleware.CORS([]string{"*"})))
	t.Cleanup(srv.Close)

	// Fixtures: a vehicle and a restricted square at a random remote spot.
	vehicle := model.Vehicle{
		ID: model.NewID(model.PrefixVehicle), VehicleNumber: "E2E-" + strings.ToUpper(model.NewID("")[:8]),
		DriverName: "Eve Driver", VehicleType: "truck", Phone: "+1234567890", Status: "active",
	}
	if err := repository.NewVehicleRepository(pool).CreateVehicle(ctx, &vehicle); err != nil {
		t.Fatal(err)
	}
	lat0, lng0 := -70+float64(time.Now().UnixNano()%1000)/100, 150.0
	geofence := model.Geofence{
		ID: model.NewID(model.PrefixGeofence), Name: "E2E Restricted", Category: model.CategoryRestrictedZone, Status: "active",
		Coordinates: []model.LatLng{{Lat: lat0, Lng: lng0}, {Lat: lat0 + 0.01, Lng: lng0}, {Lat: lat0 + 0.01, Lng: lng0 + 0.01}, {Lat: lat0, Lng: lng0}},
	}
	if err := repository.NewGeofenceRepository(pool).CreateGeofence(ctx, &geofence); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM vehicles WHERE id = $1", vehicle.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", geofence.ID)
	})

	post := func(path, body string) (*http.Response, map[string]any) {
		t.Helper()
		resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		return resp, out
	}

	// Rule via the real endpoint.
	resp, rule := post("/alerts/configure", fmt.Sprintf(`{"geofence_id":%q,"vehicle_id":%q,"event_type":"entry"}`, geofence.ID, vehicle.ID))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("configure: %d %v", resp.StatusCode, rule)
	}

	// WebSocket client.
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws/alerts", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var hello map[string]any
	_ = conn.ReadJSON(&hello)
	if hello["type"] != "connected" {
		t.Fatalf("hello = %v", hello)
	}
	for deadline := time.Now().Add(2 * time.Second); hub.ClientCount() < 1; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("client never registered")
		}
	}

	// Enter the zone.
	inLat, inLng := lat0+0.006, lng0+0.003
	ts := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	start := time.Now()
	resp, loc := post("/vehicles/location", fmt.Sprintf(`{"vehicle_id":%q,"latitude":%v,"longitude":%v,"timestamp":%q}`,
		vehicle.ID, inLat, inLng, ts.Format(time.RFC3339)))
	httpElapsed := time.Since(start)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("location: %d %v", resp.StatusCode, loc)
	}
	if httpElapsed >= matchDelay {
		t.Fatalf("POST /vehicles/location took %v (≥ %v match delay): it waited for dispatch", httpElapsed, matchDelay)
	}

	// The alert must arrive within 2s.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("no alert within 2s: %v", err)
	}
	alertElapsed := time.Since(start)
	t.Logf("HTTP returned after %v; alert arrived after %v", httpElapsed, alertElapsed)

	var alert map[string]any
	if err := json.Unmarshal(raw, &alert); err != nil {
		t.Fatal(err)
	}
	assertKeys(t, "alert", alert, "event_id", "event_type", "timestamp", "vehicle", "geofence", "location")
	assertKeys(t, "vehicle", alert["vehicle"].(map[string]any), "vehicle_id", "vehicle_number", "driver_name")
	assertKeys(t, "geofence", alert["geofence"].(map[string]any), "geofence_id", "geofence_name", "category")
	assertKeys(t, "location", alert["location"].(map[string]any), "latitude", "longitude")

	want := map[string]any{
		"event_type": "entry",
		"timestamp":  ts.Format(time.RFC3339),
		"vehicle":    map[string]any{"vehicle_id": vehicle.ID, "vehicle_number": vehicle.VehicleNumber, "driver_name": "Eve Driver"},
		"geofence":   map[string]any{"geofence_id": geofence.ID, "geofence_name": "E2E Restricted", "category": "restricted_zone"},
		"location":   map[string]any{"latitude": inLat, "longitude": inLng},
	}
	for k, v := range want {
		if fmt.Sprint(alert[k]) != fmt.Sprint(v) {
			t.Errorf("%s = %v, want %v", k, alert[k], v)
		}
	}
	eventID, _ := alert["event_id"].(string)
	if !strings.HasPrefix(eventID, "evt_") {
		t.Errorf("event_id = %q", eventID)
	}

	// Persisted and served by the history endpoint byte-for-byte as broadcast
	// (same field order, despite jsonb storage).
	histResp, err := http.Get(srv.URL + "/alerts/notifications?limit=200")
	if err != nil {
		t.Fatal(err)
	}
	defer histResp.Body.Close()
	var hist struct {
		Notifications []json.RawMessage `json:"notifications"`
	}
	_ = json.NewDecoder(histResp.Body).Decode(&hist)
	if !slices.ContainsFunc(hist.Notifications, func(p json.RawMessage) bool { return string(p) == string(raw) }) {
		t.Errorf("alert %s not in /alerts/notifications byte-for-byte:\n%s", eventID, raw)
	}
}

func assertKeys(t *testing.T, name string, m map[string]any, keys ...string) {
	t.Helper()
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	slices.Sort(got)
	want := slices.Clone(keys)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s keys = %v, want exactly %v", name, got, want)
	}
}
