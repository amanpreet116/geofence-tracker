package repository

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

func newTestGeofence(t *testing.T, repo *GeofenceRepository) model.Geofence {
	t.Helper()
	g := model.Geofence{
		ID: model.NewID(model.PrefixGeofence), Name: "alert test", Category: model.CategoryRestrictedZone,
		Status:      model.GeofenceStatusActive,
		Coordinates: []model.LatLng{{Lat: 0, Lng: 0}, {Lat: 0, Lng: 1}, {Lat: 1, Lng: 1}, {Lat: 0, Lng: 0}},
	}
	if err := repo.CreateGeofence(context.Background(), &g); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = repo.pool.Exec(context.Background(), "DELETE FROM geofences WHERE id = $1", g.ID) })
	return g
}

func newRule(geofenceID string, vehicleID *string, typ model.AlertEventType) model.AlertRule {
	return model.AlertRule{ID: model.NewID(model.PrefixAlert), GeofenceID: geofenceID, VehicleID: vehicleID, EventType: typ, Status: "active"}
}

func TestAlertRuleLifecycle(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	repo := NewAlertRepository(pool)
	g := newTestGeofence(t, NewGeofenceRepository(pool))
	v := newTestVehicle(t, NewVehicleRepository(pool))

	// Unknown references → 404 naming the missing entity.
	missingGeo := newRule("geo_missing", nil, model.AlertOnEntry)
	if err := repo.CreateRule(ctx, &missingGeo); !errors.Is(err, model.ErrNotFound) || err.Error() != `geofence "geo_missing" not found` {
		t.Errorf("missing geofence: %v", err)
	}
	missingVeh := newRule(g.ID, strPtr("veh_missing"), model.AlertOnEntry)
	if err := repo.CreateRule(ctx, &missingVeh); !errors.Is(err, model.ErrNotFound) || err.Error() != `vehicle "veh_missing" not found` {
		t.Errorf("missing vehicle: %v", err)
	}

	// Specific-vehicle and all-vehicles rules.
	specific := newRule(g.ID, &v.ID, model.AlertOnEntry)
	all := newRule(g.ID, nil, model.AlertOnBoth)
	for _, r := range []*model.AlertRule{&specific, &all} {
		if err := repo.CreateRule(ctx, r); err != nil {
			t.Fatalf("create %s: %v", r.ID, err)
		}
	}

	// Exact duplicates (including the NULL "all vehicles" one) → 409 with the existing id.
	for _, orig := range []model.AlertRule{specific, all} {
		dup := newRule(orig.GeofenceID, orig.VehicleID, orig.EventType)
		err := repo.CreateRule(ctx, &dup)
		de, ok := errors.AsType[*model.Error](err)
		if !ok || !errors.Is(err, model.ErrConflict) {
			t.Fatalf("duplicate of %s: err = %v, want conflict", orig.ID, err)
		}
		if d, _ := de.Details.(map[string]string); d["existing_alert_id"] != orig.ID {
			t.Errorf("conflict details = %v, want existing_alert_id %s", de.Details, orig.ID)
		}
	}
	// Same geofence+vehicle but different event type is not a duplicate.
	exitRule := newRule(g.ID, &v.ID, model.AlertOnExit)
	if err := repo.CreateRule(ctx, &exitRule); err != nil {
		t.Errorf("different event type rejected: %v", err)
	}

	// List with joins: the all-vehicles rule has null vehicle_id and vehicle_number.
	rules, err := repo.ListRules(ctx, service.AlertFilter{GeofenceID: g.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("got %d rules for geofence, want 3", len(rules))
	}
	for _, r := range rules {
		if r.GeofenceName != "alert test" {
			t.Errorf("geofence_name = %q", r.GeofenceName)
		}
		if r.ID == all.ID && (r.VehicleID != nil || r.VehicleNumber != nil) {
			t.Errorf("all-vehicles rule has vehicle %v / %v", r.VehicleID, r.VehicleNumber)
		}
		if r.ID == specific.ID && (r.VehicleNumber == nil || *r.VehicleNumber != v.VehicleNumber) {
			t.Errorf("vehicle_number = %v, want %s", r.VehicleNumber, v.VehicleNumber)
		}
	}
	byVehicle, err := repo.ListRules(ctx, service.AlertFilter{VehicleID: v.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(byVehicle) != 2 {
		t.Errorf("vehicle filter returned %d rules, want 2 (exact vehicle match only)", len(byVehicle))
	}

	// Deactivate, then an identical new rule is allowed; reactivating the old one conflicts.
	if r, err := repo.SetRuleStatus(ctx, specific.ID, "inactive"); err != nil || r.Status != "inactive" || r.GeofenceName == "" {
		t.Fatalf("deactivate: %+v %v", r, err)
	}
	replacement := newRule(g.ID, &v.ID, model.AlertOnEntry)
	if err := repo.CreateRule(ctx, &replacement); err != nil {
		t.Fatalf("recreate after deactivation: %v", err)
	}
	if _, err := repo.SetRuleStatus(ctx, specific.ID, "active"); !errors.Is(err, model.ErrConflict) {
		t.Errorf("reactivating duplicate: err = %v, want conflict", err)
	}

	// Delete.
	if err := repo.DeleteRule(ctx, specific.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteRule(ctx, specific.ID); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("second delete: %v", err)
	}
	if _, err := repo.SetRuleStatus(ctx, "alert_missing", "active"); !errors.Is(err, model.ErrNotFound) {
		t.Errorf("patch missing: %v", err)
	}
}

func TestCandidateRules(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	repo := NewAlertRepository(pool)
	geoRepo, vehRepo := NewGeofenceRepository(pool), NewVehicleRepository(pool)
	gA, gB := newTestGeofence(t, geoRepo), newTestGeofence(t, geoRepo)
	v1, v2 := newTestVehicle(t, vehRepo), newTestVehicle(t, vehRepo)

	create := func(r model.AlertRule) model.AlertRule {
		if err := repo.CreateRule(ctx, &r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	allOnA := create(newRule(gA.ID, nil, model.AlertOnEntry))
	v1OnA := create(newRule(gA.ID, &v1.ID, model.AlertOnExit))
	v2OnA := create(newRule(gA.ID, &v2.ID, model.AlertOnBoth)) // v2 not in batch → excluded
	v1OnB := create(newRule(gB.ID, &v1.ID, model.AlertOnBoth)) // gB not in batch → excluded
	inactive := create(newRule(gA.ID, &v1.ID, model.AlertOnBoth))
	if _, err := repo.SetRuleStatus(ctx, inactive.ID, "inactive"); err != nil {
		t.Fatal(err)
	}

	events := []model.GeofenceEvent{{VehicleID: v1.ID, Geofence: model.GeofenceRef{ID: gA.ID}, EventType: model.EventEntry}}
	got, err := repo.CandidateRules(ctx, events)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	want := []string{allOnA.ID, v1OnA.ID}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Errorf("candidates = %v, want %v (excluded: v2OnA %s, v1OnB %s, inactive %s)", ids, want, v2OnA.ID, v1OnB.ID, inactive.ID)
	}

	// The pure matcher then keeps only the entry-capable rule.
	matches := service.MatchRules(got, events)
	if len(matches) != 1 || len(matches[0].Rules) != 1 || matches[0].Rules[0].ID != allOnA.ID {
		t.Errorf("matches = %+v, want only %s", matches, allOnA.ID)
	}
}

func strPtr(s string) *string { return &s }
