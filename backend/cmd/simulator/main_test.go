package main

import (
	"math"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/demo"
)

func TestDensify(t *testing.T) {
	route := demo.Route
	const step = 100.0
	path := densify(route, step)

	if path[0] != route[0] || path[len(path)-1] != route[len(route)-1] {
		t.Fatal("path must start and end on the route endpoints")
	}
	wantPoints := int(math.Ceil(routeLength(route)/step)) + 1
	if len(path) < wantPoints-1 || len(path) > wantPoints+1 {
		t.Errorf("got %d points, want ≈%d", len(path), wantPoints)
	}
	// Consecutive points are never further apart than one step (corners
	// cut slightly shorter, never longer).
	for i := 1; i < len(path); i++ {
		if d := haversine(path[i-1], path[i]); d > step*1.001 {
			t.Fatalf("points %d→%d are %.1f m apart (> %.0f m)", i-1, i, d, step)
		}
	}
}

func TestHaversine(t *testing.T) {
	// One degree of latitude ≈ 111.2 km.
	if d := haversine(demo.Point{0, 0}, demo.Point{1, 0}); math.Abs(d-111195) > 100 {
		t.Errorf("1° latitude = %.0f m", d)
	}
}
