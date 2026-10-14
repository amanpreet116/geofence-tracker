package repository

import (
	"context"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

// TestInsertNotification covers dedupe per (event, rule) and the race where
// a rule is deleted between matching and inserting its notification.
func TestInsertNotification(t *testing.T) {
	pool := testutil.Pool(t)
	ctx := context.Background()
	g := newTestGeofence(t, NewGeofenceRepository(pool))
	v := newTestVehicle(t, NewVehicleRepository(pool))
	alerts := NewAlertRepository(pool)
	repo := NewNotificationRepository(pool)

	eventID := model.NewID(model.PrefixViolation)
	if _, err := pool.Exec(ctx, `
		INSERT INTO geofence_events (id, vehicle_id, geofence_id, event_type, location, latitude, longitude, occurred_at)
		VALUES ($1, $2, $3, 'entry', ST_SetSRID(ST_MakePoint(0.5, 0.25), 4326), 0.25, 0.5, now())`,
		eventID, v.ID, g.ID); err != nil {
		t.Fatal(err)
	}
	rule := newRule(g.ID, nil, model.AlertOnBoth)
	if err := alerts.CreateRule(ctx, &rule); err != nil {
		t.Fatal(err)
	}
	notification := func(ruleID string) model.AlertNotification {
		return model.AlertNotification{ID: model.NewID(model.PrefixEvent), AlertRuleID: ruleID, EventID: eventID}
	}

	if ok, err := repo.InsertNotification(ctx, notification(rule.ID)); err != nil || !ok {
		t.Fatalf("first insert: ok=%v err=%v", ok, err)
	}
	if ok, err := repo.InsertNotification(ctx, notification(rule.ID)); err != nil || ok {
		t.Errorf("same (event, rule) again: ok=%v err=%v, want deduped", ok, err)
	}
	// Rule deleted after matching: no alert, and not an error.
	if ok, err := repo.InsertNotification(ctx, notification("alert_deletedmeanwhile")); err != nil || ok {
		t.Errorf("deleted rule: ok=%v err=%v, want skipped without error", ok, err)
	}
	// Deleting the rule keeps already-dispatched notifications (rule id set to NULL).
	if err := alerts.DeleteRule(ctx, rule.ID); err != nil {
		t.Fatal(err)
	}
	var kept int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM alert_notifications WHERE geofence_event_id = $1 AND alert_rule_id IS NULL`, eventID).Scan(&kept)
	if kept != 1 {
		t.Errorf("notifications kept after rule deletion = %d, want 1", kept)
	}
}
