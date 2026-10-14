package repository

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

func newTestVehicle(t *testing.T, repo *VehicleRepository) model.Vehicle {
	t.Helper()
	v := model.Vehicle{
		ID:            model.NewID(model.PrefixVehicle),
		VehicleNumber: "IT-" + strings.ToUpper(model.NewID("")[:8]),
		DriverName:    "Test Driver", VehicleType: "van", Phone: "+1234567890",
		Status: model.VehicleStatusActive,
	}
	if err := repo.CreateVehicle(context.Background(), &v); err != nil {
		t.Fatalf("create vehicle: %v", err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(context.Background(), "DELETE FROM vehicles WHERE id = $1", v.ID) })
	return v
}

func TestCreateVehicleDuplicate(t *testing.T) {
	repo := NewVehicleRepository(testutil.Pool(t))
	v := newTestVehicle(t, repo)

	dup := v
	dup.ID = model.NewID(model.PrefixVehicle)
	err := repo.CreateVehicle(context.Background(), &dup)
	if !errors.Is(err, model.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestListFleetLocations(t *testing.T) {
	pool := testutil.Pool(t)
	repo := NewVehicleRepository(pool)
	geoRepo := NewGeofenceRepository(pool)
	ctx := context.Background()

	moving := newTestVehicle(t, repo)
	parked := newTestVehicle(t, repo) // never reports a location

	var geoIDs []string
	for range 2 {
		g := model.Geofence{
			ID: model.NewID(model.PrefixGeofence), Name: "fleet test", Category: model.CategoryTollZone,
			Status:      model.GeofenceStatusActive,
			Coordinates: []model.LatLng{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}, {Lat: 1, Lng: 1}, {Lat: 0, Lng: 0}},
		}
		if err := geoRepo.CreateGeofence(ctx, &g); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", g.ID) })
		geoIDs = append(geoIDs, g.ID)
	}
	slices.Sort(geoIDs)

	at := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)
	if _, err := pool.Exec(ctx, `UPDATE vehicles SET last_latitude = 0.25, last_longitude = 0.5, last_location_at = $2 WHERE id = $1`, moving.ID, at); err != nil {
		t.Fatal(err)
	}
	for _, gid := range geoIDs {
		if _, err := pool.Exec(ctx, `INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at) VALUES ($1, $2, $3)`, moving.ID, gid, at); err != nil {
			t.Fatal(err)
		}
	}

	fleet, err := repo.ListFleetLocations(ctx)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]model.FleetVehicle{}
	for _, f := range fleet {
		byID[f.ID] = f
	}

	m, ok := byID[moving.ID]
	if !ok {
		t.Fatal("moving vehicle missing")
	}
	wantLoc := model.LocationFix{LatLng: model.LatLng{Lat: 0.25, Lng: 0.5}, RecordedAt: at}
	if m.LastLocation == nil || *m.LastLocation != wantLoc {
		t.Errorf("last location = %+v, want %+v", m.LastLocation, wantLoc)
	}
	if !slices.Equal(m.CurrentGeofenceIDs, geoIDs) {
		t.Errorf("geofence ids = %v, want %v", m.CurrentGeofenceIDs, geoIDs)
	}

	p, ok := byID[parked.ID]
	if !ok {
		t.Fatal("parked vehicle missing")
	}
	if p.LastLocation != nil || p.CurrentGeofenceIDs == nil || len(p.CurrentGeofenceIDs) != 0 {
		t.Errorf("parked = location %+v, ids %#v; want nil and empty slice", p.LastLocation, p.CurrentGeofenceIDs)
	}
}
