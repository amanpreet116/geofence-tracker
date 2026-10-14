package service

import (
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

func strPtr(s string) *string { return &s }

func TestValidateConfigureAlert(t *testing.T) {
	tests := []struct {
		name        string
		in          ConfigureAlertInput
		wantVehicle *string
		wantFields  map[string]string
	}{
		{name: "specific vehicle", in: ConfigureAlertInput{GeofenceID: "geo_1", VehicleID: strPtr("veh_1"), EventType: "entry"}, wantVehicle: strPtr("veh_1")},
		{name: "vehicle omitted = all", in: ConfigureAlertInput{GeofenceID: "geo_1", EventType: "both"}},
		{name: "blank vehicle = all", in: ConfigureAlertInput{GeofenceID: "geo_1", VehicleID: strPtr("  "), EventType: "exit"}},
		{name: "trimmed", in: ConfigureAlertInput{GeofenceID: " geo_1 ", VehicleID: strPtr(" veh_1 "), EventType: " exit "}, wantVehicle: strPtr("veh_1")},
		{
			name:       "missing everything",
			in:         ConfigureAlertInput{},
			wantFields: map[string]string{"geofence_id": "is required", "event_type": "is required"},
		},
		{
			name:       "bad event type",
			in:         ConfigureAlertInput{GeofenceID: "geo_1", EventType: "ENTRY"},
			wantFields: map[string]string{"event_type": "must be one of entry, exit, both"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule, err := ValidateConfigureAlert(tt.in)
			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if rule.GeofenceID != "geo_1" || rule.Status != "active" || !rule.EventType.Valid() {
				t.Errorf("rule = %+v", rule)
			}
			if (rule.VehicleID == nil) != (tt.wantVehicle == nil) || (rule.VehicleID != nil && *rule.VehicleID != *tt.wantVehicle) {
				t.Errorf("vehicle = %v, want %v", rule.VehicleID, tt.wantVehicle)
			}
		})
	}
}

func TestValidateAlertStatus(t *testing.T) {
	for raw, wantErr := range map[string]bool{"active": false, " inactive ": false, "": true, "paused": true, "ACTIVE": true} {
		if _, err := ValidateAlertStatus(raw); (err != nil) != wantErr {
			t.Errorf("ValidateAlertStatus(%q) err = %v, wantErr %v", raw, err, wantErr)
		}
	}
}

func TestRuleMatches(t *testing.T) {
	event := model.GeofenceEvent{
		EventType: model.EventEntry,
		VehicleID: "veh_1",
		Geofence:  model.GeofenceRef{ID: "geo_1"},
	}
	rule := func(mutate func(r *model.AlertRule)) model.AlertRule {
		r := model.AlertRule{GeofenceID: "geo_1", VehicleID: strPtr("veh_1"), EventType: model.AlertOnEntry, Status: "active"}
		mutate(&r)
		return r
	}

	tests := []struct {
		name string
		rule model.AlertRule
		want bool
	}{
		{"exact match", rule(func(*model.AlertRule) {}), true},
		{"all vehicles", rule(func(r *model.AlertRule) { r.VehicleID = nil }), true},
		{"both matches entry", rule(func(r *model.AlertRule) { r.EventType = model.AlertOnBoth }), true},
		{"other geofence", rule(func(r *model.AlertRule) { r.GeofenceID = "geo_2" }), false},
		{"other vehicle", rule(func(r *model.AlertRule) { r.VehicleID = strPtr("veh_2") }), false},
		{"exit rule on entry event", rule(func(r *model.AlertRule) { r.EventType = model.AlertOnExit }), false},
		{"inactive", rule(func(r *model.AlertRule) { r.Status = "inactive" }), false},
		{"inactive all-vehicles both", rule(func(r *model.AlertRule) { r.Status, r.VehicleID, r.EventType = "inactive", nil, model.AlertOnBoth }), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuleMatches(tt.rule, event); got != tt.want {
				t.Errorf("RuleMatches = %v, want %v", got, tt.want)
			}
		})
	}

	exit := event
	exit.EventType = model.EventExit
	if !RuleMatches(rule(func(r *model.AlertRule) { r.EventType = model.AlertOnBoth }), exit) {
		t.Error("both should match exit")
	}
	if !RuleMatches(rule(func(r *model.AlertRule) { r.EventType = model.AlertOnExit }), exit) {
		t.Error("exit should match exit")
	}
}

func TestMatchRules(t *testing.T) {
	ev := func(id, vehicle, geofence string, typ model.EventType) model.GeofenceEvent {
		return model.GeofenceEvent{ID: id, VehicleID: vehicle, Geofence: model.GeofenceRef{ID: geofence}, EventType: typ}
	}
	events := []model.GeofenceEvent{
		ev("e1", "veh_1", "geo_A", model.EventExit),
		ev("e2", "veh_1", "geo_B", model.EventEntry),
		ev("e3", "veh_2", "geo_A", model.EventEntry),
	}
	// Candidate superset as the SQL returns it: rule r3 (veh_1 on geo_A
	// entry) is a candidate because veh_1 and geo_A both appear, but no
	// event has that exact combination.
	rules := []model.AlertRule{
		{ID: "r1", GeofenceID: "geo_A", EventType: model.AlertOnBoth, Status: "active"}, // all vehicles
		{ID: "r2", GeofenceID: "geo_B", VehicleID: strPtr("veh_1"), EventType: model.AlertOnEntry, Status: "active"},
		{ID: "r3", GeofenceID: "geo_A", VehicleID: strPtr("veh_1"), EventType: model.AlertOnEntry, Status: "active"},
	}

	got := MatchRules(rules, events)
	want := map[string][]string{"e1": {"r1"}, "e2": {"r2"}, "e3": {"r1"}}
	if len(got) != len(want) {
		t.Fatalf("got %d matches, want %d: %+v", len(got), len(want), got)
	}
	for i, m := range got {
		if m.Event.ID != events[i].ID {
			t.Errorf("match %d is for %s, want event order preserved", i, m.Event.ID)
		}
		var ruleIDs []string
		for _, r := range m.Rules {
			ruleIDs = append(ruleIDs, r.ID)
		}
		if w := want[m.Event.ID]; len(ruleIDs) != len(w) || ruleIDs[0] != w[0] {
			t.Errorf("event %s matched %v, want %v", m.Event.ID, ruleIDs, w)
		}
	}

	if got := MatchRules(rules, []model.GeofenceEvent{ev("e4", "veh_9", "geo_Z", model.EventEntry)}); got != nil {
		t.Errorf("unmatched event produced %+v", got)
	}
}
