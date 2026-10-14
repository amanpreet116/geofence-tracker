package demo

import "testing"

// inside is a ray-casting point-in-polygon test (interior points only).
func inside(p Point, ring []Point) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			in = !in
		}
	}
	return in
}

func zone(t *testing.T, name string) []Point {
	t.Helper()
	for _, g := range Geofences {
		if g.Name == name {
			return g.Coordinates
		}
	}
	t.Fatalf("no geofence %q", name)
	return nil
}

// TestRouteCrossesZones pins the demo story so the video shows every
// transition: Mission → Downtown → overlap → SoMa → restricted → out.
func TestRouteCrossesZones(t *testing.T) {
	checks := []struct {
		waypoint int
		in       []string
		notIn    []string
	}{
		{0, []string{"Mission District Customers"}, nil},
		{2, []string{"Downtown Delivery Zone"}, []string{"SoMa Warehouse District"}},
		{3, []string{"Downtown Delivery Zone", "SoMa Warehouse District"}, nil},
		{4, []string{"SoMa Warehouse District"}, []string{"Downtown Delivery Zone"}},
		{6, []string{RestrictedZone}, nil},
		{7, []string{RestrictedZone}, nil},
		{8, nil, []string{RestrictedZone}},
	}
	for _, c := range checks {
		p := Route[c.waypoint]
		for _, name := range c.in {
			if !inside(p, zone(t, name)) {
				t.Errorf("waypoint %d %v should be inside %s", c.waypoint, p, name)
			}
		}
		for _, name := range c.notIn {
			if inside(p, zone(t, name)) {
				t.Errorf("waypoint %d %v should be outside %s", c.waypoint, p, name)
			}
		}
	}
	if !inside(BridgeRoute[2], zone(t, "Bay Bridge Toll Plaza")) {
		t.Error("bridge route misses the toll plaza")
	}
}

func TestGeofencesClosedAndCategoriesCovered(t *testing.T) {
	cats := map[string]bool{}
	for _, g := range Geofences {
		if len(g.Coordinates) < 4 || g.Coordinates[0] != g.Coordinates[len(g.Coordinates)-1] {
			t.Errorf("%s: ring not closed or too short", g.Name)
		}
		cats[g.Category] = true
	}
	for _, c := range []string{"delivery_zone", "restricted_zone", "toll_zone", "customer_area"} {
		if !cats[c] {
			t.Errorf("category %s not covered", c)
		}
	}
}
