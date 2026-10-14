package repository

import (
	"cmp"
	"context"
	"slices"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

func TestWhereBuilder(t *testing.T) {
	w := &where{}
	if w.sql() != "" {
		t.Errorf("empty where = %q", w.sql())
	}
	w.add("a = %s", "x'; DROP TABLE vehicles; --")
	w.add("b >= %s", 5)
	limit := w.arg(10)
	if got := w.sql() + " LIMIT " + limit; got != " WHERE a = $1 AND b >= $2 LIMIT $3" {
		t.Errorf("sql = %q", got)
	}
	if len(w.args) != 3 || w.args[0] != "x'; DROP TABLE vehicles; --" {
		t.Errorf("args = %v (values must only travel as parameters)", w.args)
	}
}

// TestViolationHistory seeds 2 vehicles x 2 geofences with 10 events at
// known times (including a timestamp tie) and checks filters, ordering,
// pagination and total_count. Every query is scoped to this test's own
// vehicles or geofences so other data in the database can't interfere.
func TestViolationHistory(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	repo := NewViolationRepository(pool)

	vehRepo, geoRepo := NewVehicleRepository(pool), NewGeofenceRepository(pool)
	v1, v2 := newTestVehicle(t, vehRepo), newTestVehicle(t, vehRepo)
	gA, gB := newTestGeofence(t, geoRepo), newTestGeofence(t, geoRepo)

	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	type seed struct {
		vehicle, geofence string
		typ               model.EventType
		at                time.Time
	}
	var seeds []seed
	for i := range 6 { // v1: alternating A entry/exit at day 0..5
		typ := model.EventEntry
		if i%2 == 1 {
			typ = model.EventExit
		}
		seeds = append(seeds, seed{v1.ID, gA.ID, typ, base.Add(time.Duration(i) * 24 * time.Hour)})
	}
	seeds = append(seeds,
		seed{v1.ID, gB.ID, model.EventEntry, base.Add(5 * 24 * time.Hour)}, // ties with v1/A day 5
		seed{v2.ID, gA.ID, model.EventEntry, base.Add(2 * 24 * time.Hour)},
		seed{v2.ID, gB.ID, model.EventEntry, base.Add(3 * 24 * time.Hour)},
		seed{v2.ID, gB.ID, model.EventExit, base.Add(4 * 24 * time.Hour)},
	)
	byID := map[string]seed{}
	for _, s := range seeds {
		id := model.NewID(model.PrefixViolation)
		byID[id] = s
		if _, err := pool.Exec(ctx, `
			INSERT INTO geofence_events (id, vehicle_id, geofence_id, event_type, location, latitude, longitude, occurred_at)
			VALUES ($1, $2, $3, $4, ST_SetSRID(ST_MakePoint(0.5, 0.25), 4326), 0.25, 0.5, $5)`,
			id, s.vehicle, s.geofence, string(s.typ), s.at); err != nil {
			t.Fatal(err)
		}
	}
	day := func(d int) *time.Time { t := base.Add(time.Duration(d) * 24 * time.Hour); return &t }

	tests := []struct {
		name      string
		filter    service.ViolationFilter
		wantTotal int
		wantPage  int
	}{
		{name: "vehicle", filter: service.ViolationFilter{VehicleID: v1.ID, Limit: 50}, wantTotal: 7, wantPage: 7},
		{name: "vehicle + geofence", filter: service.ViolationFilter{VehicleID: v1.ID, GeofenceID: gA.ID, Limit: 50}, wantTotal: 6, wantPage: 6},
		{name: "geofence only (both vehicles)", filter: service.ViolationFilter{GeofenceID: gB.ID, Limit: 50}, wantTotal: 3, wantPage: 3},
		{name: "vehicle + start", filter: service.ViolationFilter{VehicleID: v1.ID, Start: day(4), Limit: 50}, wantTotal: 3, wantPage: 3},
		{name: "vehicle + end (inclusive)", filter: service.ViolationFilter{VehicleID: v1.ID, End: day(1), Limit: 50}, wantTotal: 2, wantPage: 2},
		{name: "vehicle + range", filter: service.ViolationFilter{VehicleID: v2.ID, Start: day(3), End: day(4), Limit: 50}, wantTotal: 2, wantPage: 2},
		{name: "all four filters", filter: service.ViolationFilter{VehicleID: v2.ID, GeofenceID: gB.ID, Start: day(4), End: day(4), Limit: 50}, wantTotal: 1, wantPage: 1},
		{name: "limit smaller than total", filter: service.ViolationFilter{VehicleID: v1.ID, Limit: 3}, wantTotal: 7, wantPage: 3},
		{name: "offset", filter: service.ViolationFilter{VehicleID: v1.ID, Limit: 3, Offset: 6}, wantTotal: 7, wantPage: 1},
		{name: "offset past the end keeps total", filter: service.ViolationFilter{VehicleID: v1.ID, Limit: 3, Offset: 100}, wantTotal: 7, wantPage: 0},
		{name: "no match", filter: service.ViolationFilter{VehicleID: v1.ID, Start: day(30), Limit: 50}, wantTotal: 0, wantPage: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, total, err := repo.ListViolations(ctx, tt.filter)
			if err != nil {
				t.Fatal(err)
			}
			if total != tt.wantTotal || len(page) != tt.wantPage {
				t.Fatalf("total=%d page=%d, want total=%d page=%d", total, len(page), tt.wantTotal, tt.wantPage)
			}
			for i, e := range page {
				s := byID[e.ID]
				if e.VehicleID != s.vehicle || e.Geofence.ID != s.geofence || e.EventType != s.typ || !e.OccurredAt.Equal(s.at) {
					t.Errorf("row %d = %+v does not match seed %+v", i, e, s)
				}
				if e.VehicleNumber == "" || e.Geofence.Name != "alert test" || e.Location != (model.LatLng{Lat: 0.25, Lng: 0.5}) {
					t.Errorf("joins/location wrong: %+v", e)
				}
				if f := tt.filter; (f.Start != nil && e.OccurredAt.Before(*f.Start)) || (f.End != nil && e.OccurredAt.After(*f.End)) {
					t.Errorf("row %s at %v outside date range", e.ID, e.OccurredAt)
				}
			}
		})
	}

	// Paging through 3 at a time yields every row exactly once, in
	// (occurred_at DESC, id DESC) order, including the day-5 tie.
	all, _, err := repo.ListViolations(ctx, service.ViolationFilter{VehicleID: v1.ID, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.IsSortedFunc(all, func(a, b model.GeofenceEvent) int {
		if c := b.OccurredAt.Compare(a.OccurredAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	}) {
		t.Error("not ordered by occurred_at DESC, id DESC")
	}
	var paged []string
	for offset := 0; ; offset += 3 {
		page, total, err := repo.ListViolations(ctx, service.ViolationFilter{VehicleID: v1.ID, Limit: 3, Offset: offset})
		if err != nil {
			t.Fatal(err)
		}
		if total != 7 {
			t.Fatalf("total at offset %d = %d", offset, total)
		}
		if len(page) == 0 {
			break
		}
		for _, e := range page {
			paged = append(paged, e.ID)
		}
	}
	var want []string
	for _, e := range all {
		want = append(want, e.ID)
	}
	if !slices.Equal(paged, want) {
		t.Errorf("paged ids %v != full list %v", paged, want)
	}
}
