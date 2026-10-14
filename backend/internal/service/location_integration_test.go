package service_test

import (
	"context"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/repository"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

// fixture is one vehicle and one square geofence placed at a random spot
// in the Southern Ocean, so parallel test runs never overlap each other or
// demo data.
type fixture struct {
	pool      *pgxpool.Pool
	svc       *service.LocationService
	vehicleID string
	geofence  string
	// square spans [lat0, lat0+size] x [lng0, lng0+size]
	lat0, lng0, size float64
	t0               time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	pool := testutil.Pool(t)
	ctx := context.Background()
	f := &fixture{
		pool: pool,
		svc:  service.NewLocationService(repository.NewLocationRepository(pool), service.NoopDispatcher{}),
		lat0: -60 + rand.Float64()*10, lng0: 100 + rand.Float64()*10, size: 0.01,
		t0: time.Now().UTC().Add(-time.Hour).Truncate(time.Second),
	}

	v := model.Vehicle{
		ID: model.NewID(model.PrefixVehicle), VehicleNumber: "IT-" + strings.ToUpper(model.NewID("")[:8]),
		DriverName: "Test", VehicleType: "van", Phone: "+1234567890", Status: model.VehicleStatusActive,
	}
	if err := repository.NewVehicleRepository(pool).CreateVehicle(ctx, &v); err != nil {
		t.Fatal(err)
	}
	g := model.Geofence{
		ID: model.NewID(model.PrefixGeofence), Name: "IT square", Category: model.CategoryRestrictedZone,
		Status: model.GeofenceStatusActive,
		Coordinates: []model.LatLng{
			{Lat: f.lat0, Lng: f.lng0}, {Lat: f.lat0 + f.size, Lng: f.lng0},
			{Lat: f.lat0 + f.size, Lng: f.lng0 + f.size}, {Lat: f.lat0, Lng: f.lng0 + f.size},
			{Lat: f.lat0, Lng: f.lng0},
		},
	}
	if err := repository.NewGeofenceRepository(pool).CreateGeofence(ctx, &g); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM vehicles WHERE id = $1", v.ID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", g.ID)
	})
	f.vehicleID, f.geofence = v.ID, g.ID
	return f
}

func (f *fixture) inside() (lat, lng float64)  { return f.lat0 + f.size/2, f.lng0 + f.size/2 }
func (f *fixture) outside() (lat, lng float64) { return f.lat0 - f.size/2, f.lng0 + f.size/2 }

func (f *fixture) post(t *testing.T, lat, lng float64, at time.Time) service.LocationResult {
	t.Helper()
	res, err := f.svc.UpdateLocation(context.Background(), service.UpdateLocationInput{
		VehicleID: f.vehicleID, Latitude: &lat, Longitude: &lng, Timestamp: at.Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("update at %v: %v", at, err)
	}
	return res
}

func (f *fixture) isInside(res service.LocationResult) bool {
	return slices.ContainsFunc(res.CurrentGeofences, func(g model.GeofenceRef) bool { return g.ID == f.geofence })
}

type storedEvent struct {
	Type string
	At   time.Time
}

// events returns this fixture's geofence events in time order.
func (f *fixture) events(t *testing.T) []storedEvent {
	t.Helper()
	rows, err := f.pool.Query(context.Background(), `
		SELECT event_type, occurred_at FROM geofence_events
		WHERE vehicle_id = $1 AND geofence_id = $2 ORDER BY occurred_at, id`, f.vehicleID, f.geofence)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []storedEvent
	for rows.Next() {
		var e storedEvent
		if err := rows.Scan(&e.Type, &e.At); err != nil {
			t.Fatal(err)
		}
		out = append(out, storedEvent{e.Type, e.At.UTC()})
	}
	return out
}

func TestEntryExitSequence(t *testing.T) {
	f := newFixture(t)
	steps := []struct {
		inside     bool
		wantInside bool
		wantEvents int
	}{
		{inside: false, wantInside: false, wantEvents: 0}, // outside
		{inside: true, wantInside: true, wantEvents: 1},   // entry
		{inside: true, wantInside: true, wantEvents: 0},   // still inside
		{inside: false, wantInside: false, wantEvents: 1}, // exit
	}
	for i, s := range steps {
		lat, lng := f.outside()
		if s.inside {
			lat, lng = f.inside()
		}
		res := f.post(t, lat, lng, f.t0.Add(time.Duration(i)*time.Minute))
		if f.isInside(res) != s.wantInside || len(res.Events) != s.wantEvents {
			t.Fatalf("step %d: inside=%v events=%d, want %v/%d", i, f.isInside(res), len(res.Events), s.wantInside, s.wantEvents)
		}
	}

	got := f.events(t)
	want := []storedEvent{{"entry", f.t0.Add(time.Minute)}, {"exit", f.t0.Add(3 * time.Minute)}}
	if !slices.Equal(got, want) {
		t.Errorf("stored events = %v, want %v", got, want)
	}

	var history int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM vehicle_locations WHERE vehicle_id = $1`, f.vehicleID).Scan(&history)
	if history != 4 {
		t.Errorf("history rows = %d, want 4", history)
	}
}

func TestBoundaryCountsAsInside(t *testing.T) {
	f := newFixture(t)
	points := []struct {
		name     string
		lat, lng float64
	}{
		{"on bottom edge", f.lat0, f.lng0 + f.size/2},
		{"on right edge", f.lat0 + f.size/2, f.lng0 + f.size},
		{"on a vertex", f.lat0 + f.size, f.lng0 + f.size},
	}
	for i, p := range points {
		res := f.post(t, p.lat, p.lng, f.t0.Add(time.Duration(i)*time.Minute))
		if !f.isInside(res) {
			t.Errorf("%s: not reported inside", p.name)
		}
	}
	if got := f.events(t); len(got) != 1 || got[0].Type != "entry" {
		t.Errorf("events = %v, want a single entry (moving along the boundary is not a transition)", got)
	}
}

func TestOutOfOrderUpdateIsHistoryOnly(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	inLat, inLng := f.inside()
	f.post(t, inLat, inLng, f.t0.Add(10*time.Minute)) // entry

	outLat, outLng := f.outside()
	res := f.post(t, outLat, outLng, f.t0.Add(5*time.Minute)) // older: must be ignored for state
	if !res.Stale || len(res.Events) != 0 || !f.isInside(res) {
		t.Fatalf("stale=%v events=%d inside=%v, want stale, no events, still inside", res.Stale, len(res.Events), f.isInside(res))
	}

	if got := f.events(t); len(got) != 1 || got[0].Type != "entry" {
		t.Errorf("events = %v, want only the original entry", got)
	}
	vl, err := f.svc.GetVehicleLocation(ctx, f.vehicleID)
	if err != nil {
		t.Fatal(err)
	}
	if vl.LastLocation == nil || !vl.LastLocation.RecordedAt.Equal(f.t0.Add(10*time.Minute)) || vl.LastLocation.Lat != inLat {
		t.Errorf("last location changed by stale update: %+v", vl.LastLocation)
	}
	var history int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM vehicle_locations WHERE vehicle_id = $1`, f.vehicleID).Scan(&history)
	if history != 2 {
		t.Errorf("history rows = %d, want 2 (stale report still stored)", history)
	}
}

func TestGetVehicleLocation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	vl, err := f.svc.GetVehicleLocation(ctx, f.vehicleID)
	if err != nil {
		t.Fatal(err)
	}
	if vl.LastLocation != nil || len(vl.CurrentGeofences) != 0 {
		t.Errorf("before any report: %+v, want no location and no geofences", vl)
	}

	lat, lng := f.inside()
	at := f.t0.Add(time.Minute)
	f.post(t, lat, lng, at)
	vl, err = f.svc.GetVehicleLocation(ctx, f.vehicleID)
	if err != nil {
		t.Fatal(err)
	}
	want := model.LocationFix{LatLng: model.LatLng{Lat: lat, Lng: lng}, RecordedAt: at}
	if vl.LastLocation == nil || *vl.LastLocation != want {
		t.Errorf("last location = %+v, want %+v", vl.LastLocation, want)
	}
	if len(vl.CurrentGeofences) != 1 || vl.CurrentGeofences[0].ID != f.geofence ||
		vl.CurrentGeofences[0].Category != model.CategoryRestrictedZone {
		t.Errorf("current geofences = %+v", vl.CurrentGeofences)
	}
}

// TestConcurrentUpdates fires 20 updates for one vehicle at once,
// alternating inside/outside with increasing timestamps. Arrival order is
// random, so some become stale. Whatever the interleaving, the stored
// events must equal exactly the transitions of the applied (non-stale)
// updates in timestamp order — no duplicated or missing entry/exit.
func TestConcurrentUpdates(t *testing.T) {
	f := newFixture(t)
	const n = 20

	type outcome struct {
		at     time.Time
		inside bool
		stale  bool
	}
	results := make([]outcome, n)
	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
		errs  = make(chan error, n)
	)
	for i := range n {
		wg.Go(func() {
			lat, lng := f.outside()
			if i%2 == 0 {
				lat, lng = f.inside()
			}
			at := f.t0.Add(time.Duration(i) * time.Second)
			<-start
			res, err := f.svc.UpdateLocation(context.Background(), service.UpdateLocationInput{
				VehicleID: f.vehicleID, Latitude: &lat, Longitude: &lng, Timestamp: at.Format(time.RFC3339),
			})
			if err != nil {
				errs <- err
				return
			}
			results[i] = outcome{at: at, inside: i%2 == 0, stale: res.Stale}
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("update failed: %v", err)
	}

	// Replay the applied updates in timestamp order to derive the expected events.
	var applied []outcome
	for _, r := range results {
		if !r.stale {
			applied = append(applied, r)
		}
	}
	slices.SortFunc(applied, func(a, b outcome) int { return a.at.Compare(b.at) })
	var want []storedEvent
	wasInside := false
	for _, a := range applied {
		switch {
		case a.inside && !wasInside:
			want = append(want, storedEvent{"entry", a.at})
		case !a.inside && wasInside:
			want = append(want, storedEvent{"exit", a.at})
		}
		wasInside = a.inside
	}

	got := f.events(t)
	if !slices.Equal(got, want) {
		t.Fatalf("events = %v\nwant     %v\n(applied %d of %d updates)", got, want, len(applied), n)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Type == got[i-1].Type {
			t.Fatalf("two consecutive %q events at %v and %v", got[i].Type, got[i-1].At, got[i].At)
		}
	}

	// Current state must agree with the last applied update.
	ctx := context.Background()
	vl, err := f.svc.GetVehicleLocation(ctx, f.vehicleID)
	if err != nil {
		t.Fatal(err)
	}
	last := applied[len(applied)-1]
	insideNow := slices.ContainsFunc(vl.CurrentGeofences, func(g model.GeofenceRef) bool { return g.ID == f.geofence })
	if !vl.LastLocation.RecordedAt.Equal(last.at) || insideNow != last.inside {
		t.Errorf("state = (at %v, inside %v), want (at %v, inside %v)", vl.LastLocation.RecordedAt, insideNow, last.at, last.inside)
	}
	var history int
	_ = f.pool.QueryRow(ctx, `SELECT count(*) FROM vehicle_locations WHERE vehicle_id = $1`, f.vehicleID).Scan(&history)
	if history != n {
		t.Errorf("history rows = %d, want %d", history, n)
	}
	t.Logf("%d of %d concurrent updates applied in order, %d stale; %d events", len(applied), n, n-len(applied), len(got))
}

// TestOverlappingAndNestedGeofences: a point inside two overlapping squares
// is inside both; moving to the part covered by only one exits the other.
func TestOverlappingAndNestedGeofences(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// Second square overlapping the fixture's upper-right quarter, and a
	// small square nested fully inside that overlap.
	mk := func(name string, lat0, lng0, size float64) string {
		g := model.Geofence{
			ID: model.NewID(model.PrefixGeofence), Name: name, Category: model.CategoryTollZone, Status: "active",
			Coordinates: []model.LatLng{
				{Lat: lat0, Lng: lng0}, {Lat: lat0 + size, Lng: lng0}, {Lat: lat0 + size, Lng: lng0 + size},
				{Lat: lat0, Lng: lng0 + size}, {Lat: lat0, Lng: lng0},
			},
		}
		if err := repository.NewGeofenceRepository(f.pool).CreateGeofence(ctx, &g); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = f.pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", g.ID) })
		return g.ID
	}
	overlap := mk("IT overlap", f.lat0+f.size/2, f.lng0+f.size/2, f.size)
	nested := mk("IT nested", f.lat0+f.size*0.7, f.lng0+f.size*0.7, f.size*0.1)

	set := func(res service.LocationResult) []string {
		var ids []string
		for _, g := range res.CurrentGeofences {
			if g.ID == f.geofence || g.ID == overlap || g.ID == nested {
				ids = append(ids, g.ID)
			}
		}
		slices.Sort(ids)
		return ids
	}
	sorted := func(ids ...string) []string { slices.Sort(ids); return ids }

	// Inside all three (nested square sits in the overlap).
	res := f.post(t, f.lat0+f.size*0.75, f.lng0+f.size*0.75, f.t0)
	if got, want := set(res), sorted(f.geofence, overlap, nested); !slices.Equal(got, want) {
		t.Fatalf("inside = %v, want all three %v", got, want)
	}
	if len(res.Events) != 3 {
		t.Errorf("events = %d, want 3 entries", len(res.Events))
	}
	// Overlap only, outside the nested one: a single exit.
	res = f.post(t, f.lat0+f.size*0.9, f.lng0+f.size*0.9, f.t0.Add(time.Minute))
	if got, want := set(res), sorted(f.geofence, overlap); !slices.Equal(got, want) {
		t.Errorf("inside = %v, want %v", got, want)
	}
	if len(res.Events) != 1 || res.Events[0].EventType != model.EventExit || res.Events[0].Geofence.ID != nested {
		t.Errorf("events = %+v, want one exit from the nested zone", res.Events)
	}
}

// TestDuplicatePostIsIdempotent: the same report twice stores one history
// row and emits no second set of events.
func TestDuplicatePostIsIdempotent(t *testing.T) {
	f := newFixture(t)
	lat, lng := f.inside()
	first := f.post(t, lat, lng, f.t0)
	second := f.post(t, lat, lng, f.t0)
	if len(first.Events) != 1 || len(second.Events) != 0 || !second.Duplicate || !f.isInside(second) {
		t.Errorf("first events=%d, second events=%d duplicate=%v inside=%v", len(first.Events), len(second.Events), second.Duplicate, f.isInside(second))
	}
	var history int
	_ = f.pool.QueryRow(context.Background(), `SELECT count(*) FROM vehicle_locations WHERE vehicle_id = $1`, f.vehicleID).Scan(&history)
	if history != 1 {
		t.Errorf("history rows = %d, want 1", history)
	}
}

// TestNoGeofencesNearby: a vehicle far from every geofence is simply outside.
func TestNoGeofencesNearby(t *testing.T) {
	f := newFixture(t)
	res := f.post(t, f.lat0-5, f.lng0-5, f.t0) // nothing within hundreds of km
	if len(res.Events) != 0 || f.isInside(res) {
		t.Errorf("events=%d inside=%v", len(res.Events), f.isInside(res))
	}
}
