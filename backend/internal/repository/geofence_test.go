package repository

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

func TestApiOrderReason(t *testing.T) {
	tests := map[string]string{
		"Self-intersection[-122.41 37.77]": "Self-intersection at [37.77, -122.41]",
		"Ring Self-intersection[1 0.5]":    "Ring Self-intersection at [0.5, 1]",
		"Valid Geometry":                   "Valid Geometry",
		"Too few points":                   "Too few points",
	}
	for in, want := range tests {
		if got := apiOrderReason(in); got != want {
			t.Errorf("apiOrderReason(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGeofenceRoundTrip(t *testing.T) {
	pool := testutil.Pool(t)
	repo := NewGeofenceRepository(pool)
	ctx := context.Background()

	// Awkward doubles that would be altered by decimal rounding (e.g.
	// ST_AsGeoJSON's default 9 digits) or by a lat/lng swap.
	ring := []model.LatLng{
		{Lat: 37.774929999999996, Lng: -122.41941234567891},
		{Lat: 37.78490000000001, Lng: -122.4194},
		{Lat: 37.7849, Lng: -122.40940000000003},
		{Lat: 0.1 + 0.2, Lng: -122.4094},
		{Lat: 37.774929999999996, Lng: -122.41941234567891},
	}
	in := model.Geofence{
		ID:          model.NewID(model.PrefixGeofence),
		Name:        "round trip",
		Description: "", // stored as NULL, read back as ""
		Category:    model.CategoryRestrictedZone,
		Coordinates: ring,
		Status:      model.GeofenceStatusActive,
	}
	if err := repo.CreateGeofence(ctx, &in); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", in.ID) })
	if in.CreatedAt.IsZero() {
		t.Error("CreatedAt not populated")
	}

	got, err := repo.GetGeofence(ctx, in.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !slices.Equal(got.Coordinates, ring) {
		t.Errorf("coordinates changed in round trip:\n got  %v\n want %v", got.Coordinates, ring)
	}
	if got.Name != in.Name || got.Description != "" || got.Category != in.Category || got.Status != "active" ||
		!got.CreatedAt.Equal(in.CreatedAt) || got.CreatedAt.Location().String() != "UTC" {
		t.Errorf("fields changed in round trip: %+v", got)
	}

	// Stored in PostGIS order: x = longitude.
	var x, y float64
	if err := pool.QueryRow(ctx, `SELECT ST_X(ST_PointN(ST_ExteriorRing(boundary), 1)), ST_Y(ST_PointN(ST_ExteriorRing(boundary), 1))
		FROM geofences WHERE id = $1`, in.ID).Scan(&x, &y); err != nil {
		t.Fatal(err)
	}
	if x != ring[0].Lng || y != ring[0].Lat {
		t.Errorf("stored (x, y) = (%v, %v), want (lng, lat) = (%v, %v)", x, y, ring[0].Lng, ring[0].Lat)
	}

	// Category filter includes it; another category excludes it.
	restricted, err := repo.ListGeofences(ctx, model.CategoryRestrictedZone)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !containsID(restricted, in.ID) {
		t.Error("filtered list is missing the geofence")
	}
	tolls, err := repo.ListGeofences(ctx, model.CategoryTollZone)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if containsID(tolls, in.ID) {
		t.Error("geofence leaked into another category")
	}
}

func TestGetGeofenceNotFound(t *testing.T) {
	repo := NewGeofenceRepository(testutil.Pool(t))
	if _, err := repo.GetGeofence(context.Background(), "geo_missing"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestCheckPolygon(t *testing.T) {
	repo := NewGeofenceRepository(testutil.Pool(t))

	tests := []struct {
		name       string
		ring       []model.LatLng
		wantValid  bool
		wantReason string
	}{
		{
			name:      "square",
			ring:      []model.LatLng{ll(0, 0), ll(0, 1), ll(1, 1), ll(1, 0), ll(0, 0)},
			wantValid: true, wantReason: "Valid Geometry",
		},
		{
			name: "bowtie",
			// (lat,lng) (0,0)->(1,1)->(1,0)->(0,1) crosses itself at (0.5, 0.5).
			ring:       []model.LatLng{ll(0, 0), ll(1, 1), ll(1, 0), ll(0, 1), ll(0, 0)},
			wantReason: "Self-intersection at [0.5, 0.5]",
		},
		{
			name: "asymmetric bowtie reports location in [lat, lng] order",
			// Edges (10,0)-(12,4) and (10,4)-(12,0) cross at lat 11, lng 2.
			ring:       []model.LatLng{ll(10, 0), ll(12, 4), ll(10, 4), ll(12, 0), ll(10, 0)},
			wantReason: "Self-intersection at [11, 2]",
		},
		{
			name:      "collinear (zero area)",
			ring:      []model.LatLng{ll(0, 0), ll(1, 1), ll(2, 2), ll(0, 0)},
			wantValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, reason, err := repo.CheckPolygon(context.Background(), tt.ring)
			if err != nil {
				t.Fatal(err)
			}
			if valid != tt.wantValid {
				t.Errorf("valid = %v, want %v (reason %q)", valid, tt.wantValid, reason)
			}
			if tt.wantReason != "" && reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
			if !tt.wantValid && reason == "" {
				t.Error("invalid polygon without a reason")
			}
		})
	}
}

func containsID(gs []model.Geofence, id string) bool {
	return slices.ContainsFunc(gs, func(g model.Geofence) bool { return g.ID == id })
}

func ll(lat, lng float64) model.LatLng { return model.LatLng{Lat: lat, Lng: lng} }
