package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

type fakeMatcher struct {
	rules []model.AlertRule
	delay time.Duration
	block bool // wait for ctx cancellation
}

func (m *fakeMatcher) Match(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertMatch, error) {
	if m.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	time.Sleep(m.delay)
	return MatchRules(m.rules, events), nil
}

type fakeStore struct {
	mu   sync.Mutex
	seen map[[2]string]bool
}

func (s *fakeStore) InsertNotification(_ context.Context, n model.AlertNotification) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [2]string{n.EventID, n.AlertRuleID}
	if s.seen[key] {
		return false, nil
	}
	s.seen[key] = true
	return true, nil
}

type fakeBroadcaster struct {
	mu   sync.Mutex
	msgs []string
}

func (b *fakeBroadcaster) Broadcast(msg []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, string(msg))
}

func (b *fakeBroadcaster) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.msgs)
}

func testEvent(id string) model.GeofenceEvent {
	return model.GeofenceEvent{
		ID: id, EventType: model.EventEntry,
		VehicleID: "veh_456", VehicleNumber: "KA-01-AB-1234", DriverName: "John Doe",
		Geofence:   model.GeofenceRef{ID: "geo_123", Name: "Downtown Delivery Zone", Category: model.CategoryDeliveryZone},
		Location:   model.LatLng{Lat: 37.7849, Lng: -122.4194},
		OccurredAt: time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
	}
}

func newTestDispatcher(m *fakeMatcher, cfg DispatcherConfig) (*AlertDispatcher, *fakeBroadcaster) {
	b := &fakeBroadcaster{}
	d := NewAlertDispatcher(m, &fakeStore{seen: map[[2]string]bool{}}, b, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return d, b
}

func TestBuildAlertPayloadExactShape(t *testing.T) {
	got, _ := json.Marshal(BuildAlertPayload("evt_999", testEvent("viol_1")))
	want := `{"event_id":"evt_999","event_type":"entry","timestamp":"2025-01-15T10:35:00Z",` +
		`"vehicle":{"vehicle_id":"veh_456","vehicle_number":"KA-01-AB-1234","driver_name":"John Doe"},` +
		`"geofence":{"geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","category":"delivery_zone"},` +
		`"location":{"latitude":37.7849,"longitude":-122.4194}}`
	if string(got) != want {
		t.Errorf("payload\n got %s\nwant %s", got, want)
	}
}

func TestDispatcherNotifiesOncePerEventRule(t *testing.T) {
	rules := []model.AlertRule{
		{ID: "alert_all", GeofenceID: "geo_123", EventType: model.AlertOnBoth, Status: "active"},
		{ID: "alert_veh", GeofenceID: "geo_123", VehicleID: strPtr("veh_456"), EventType: model.AlertOnEntry, Status: "active"},
		{ID: "alert_exit", GeofenceID: "geo_123", EventType: model.AlertOnExit, Status: "active"}, // no match
	}
	d, b := newTestDispatcher(&fakeMatcher{rules: rules}, DispatcherConfig{Workers: 2})
	d.Start()

	event := testEvent("viol_1")
	d.Dispatch([]model.GeofenceEvent{event})
	d.Dispatch([]model.GeofenceEvent{event}) // duplicate enqueue: deduped by (event, rule)
	if err := d.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}

	if b.count() != 2 {
		t.Fatalf("broadcasts = %d, want 2 (one per matching rule, despite double dispatch)", b.count())
	}
	ids := map[string]bool{}
	for _, m := range b.msgs {
		var p model.AlertPayload
		if err := json.Unmarshal([]byte(m), &p); err != nil {
			t.Fatal(err)
		}
		ids[p.EventID] = true
		if p.Vehicle.VehicleNumber != "KA-01-AB-1234" || p.Geofence.Category != "delivery_zone" {
			t.Errorf("payload = %+v", p)
		}
	}
	if len(ids) != 2 {
		t.Errorf("event ids not unique: %v", ids)
	}
}

func TestDispatchDoesNotBlock(t *testing.T) {
	d, b := newTestDispatcher(&fakeMatcher{rules: []model.AlertRule{
		{ID: "r", GeofenceID: "geo_123", EventType: model.AlertOnBoth, Status: "active"},
	}, delay: 300 * time.Millisecond}, DispatcherConfig{Workers: 1})
	d.Start()
	defer func() { _ = d.Shutdown(context.Background()) }()

	start := time.Now()
	d.Dispatch([]model.GeofenceEvent{testEvent("viol_1")})
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Errorf("Dispatch took %v; must only enqueue", elapsed)
	}
	if b.count() != 0 {
		t.Error("delivered synchronously")
	}
}

func TestQueueFullFallsBackToSynchronous(t *testing.T) {
	rule := model.AlertRule{ID: "r", GeofenceID: "geo_123", EventType: model.AlertOnBoth, Status: "active"}
	d, b := newTestDispatcher(&fakeMatcher{rules: []model.AlertRule{rule}}, DispatcherConfig{Workers: 1, QueueSize: 1})
	// Workers not started: the queue fills after one batch.
	d.Dispatch([]model.GeofenceEvent{testEvent("viol_1")}) // queued
	d.Dispatch([]model.GeofenceEvent{testEvent("viol_2")}) // queue full → processed inline
	if b.count() != 1 {
		t.Fatalf("broadcasts = %d, want 1 (overflow batch processed synchronously)", b.count())
	}
	d.Start()
	if err := d.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if b.count() != 2 {
		t.Errorf("broadcasts after drain = %d, want 2 (nothing lost)", b.count())
	}
}

func TestShutdownDrainsQueue(t *testing.T) {
	rule := model.AlertRule{ID: "r", GeofenceID: "geo_123", EventType: model.AlertOnBoth, Status: "active"}
	d, b := newTestDispatcher(&fakeMatcher{rules: []model.AlertRule{rule}, delay: 20 * time.Millisecond}, DispatcherConfig{Workers: 2})
	d.Start()
	for i := range 10 {
		d.Dispatch([]model.GeofenceEvent{testEvent("viol_" + string(rune('a'+i)))})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := d.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if b.count() != 10 {
		t.Errorf("delivered %d of 10 queued batches", b.count())
	}
	d.Dispatch([]model.GeofenceEvent{testEvent("viol_late")}) // after shutdown: ignored, no panic
}

func TestShutdownTimeoutCancelsWork(t *testing.T) {
	d, _ := newTestDispatcher(&fakeMatcher{block: true}, DispatcherConfig{Workers: 1})
	d.Start()
	d.Dispatch([]model.GeofenceEvent{testEvent("viol_1")})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	if err := d.Shutdown(ctx); err == nil {
		t.Error("expected drain timeout error")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("shutdown took %v; in-flight work was not cancelled", elapsed)
	}
}
