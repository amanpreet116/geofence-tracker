package service

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

func refs(ids ...string) []model.GeofenceRef {
	out := make([]model.GeofenceRef, len(ids))
	for i, id := range ids {
		out[i] = model.GeofenceRef{ID: id, Name: "name-" + id}
	}
	return out
}

func ids(rs []model.GeofenceRef) []string {
	out := []string{}
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func TestComputeTransitions(t *testing.T) {
	tests := []struct {
		name        string
		prev, curr  []string
		wantEntered []string
		wantExited  []string
	}{
		{name: "nothing to nothing", prev: nil, curr: nil, wantEntered: []string{}, wantExited: []string{}},
		{name: "empty to A: entry", prev: nil, curr: []string{"A"}, wantEntered: []string{"A"}, wantExited: []string{}},
		{name: "A to A: none", prev: []string{"A"}, curr: []string{"A"}, wantEntered: []string{}, wantExited: []string{}},
		{name: "A to empty: exit", prev: []string{"A"}, curr: nil, wantEntered: []string{}, wantExited: []string{"A"}},
		{name: "A to B: exit A, entry B", prev: []string{"A"}, curr: []string{"B"}, wantEntered: []string{"B"}, wantExited: []string{"A"}},
		{name: "A,B to B: exit A", prev: []string{"A", "B"}, curr: []string{"B"}, wantEntered: []string{}, wantExited: []string{"A"}},
		{
			// City contains District contains Block; driving into the block
			// from inside the city enters only the two inner zones.
			name: "nested: deeper into nested zones", prev: []string{"city"}, curr: []string{"city", "district", "block"},
			wantEntered: []string{"block", "district"}, wantExited: []string{},
		},
		{
			name: "nested: leave all at once", prev: []string{"block", "city", "district"}, curr: nil,
			wantEntered: []string{}, wantExited: []string{"block", "city", "district"},
		},
		{
			// Overlapping X and Y: moving from X-only through the overlap into Y-only.
			name: "overlap: X into X∩Y", prev: []string{"X"}, curr: []string{"X", "Y"},
			wantEntered: []string{"Y"}, wantExited: []string{},
		},
		{
			name: "overlap: X∩Y into Y", prev: []string{"Y", "X"}, curr: []string{"Y"},
			wantEntered: []string{}, wantExited: []string{"X"},
		},
		{
			name: "results sorted regardless of input order", prev: []string{"d", "a"}, curr: []string{"z", "b", "y"},
			wantEntered: []string{"b", "y", "z"}, wantExited: []string{"a", "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entered, exited := ComputeTransitions(refs(tt.prev...), refs(tt.curr...))
			if got := ids(entered); !slices.Equal(got, tt.wantEntered) {
				t.Errorf("entered = %v, want %v", got, tt.wantEntered)
			}
			if got := ids(exited); !slices.Equal(got, tt.wantExited) {
				t.Errorf("exited = %v, want %v", got, tt.wantExited)
			}
		})
	}
}

func ptr(f float64) *float64 { return &f }

func TestValidateLocationUpdate(t *testing.T) {
	now := time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)
	valid := func() UpdateLocationInput {
		return UpdateLocationInput{VehicleID: "veh_456", Latitude: ptr(37.7849), Longitude: ptr(-122.4194), Timestamp: "2025-01-15T10:35:00Z"}
	}

	tests := []struct {
		name       string
		mutate     func(in *UpdateLocationInput)
		wantFields map[string]string
		wantTime   time.Time
	}{
		{name: "spec example", mutate: func(*UpdateLocationInput) {}, wantTime: now},
		{name: "zero lat/lng is a real position", mutate: func(in *UpdateLocationInput) { in.Latitude, in.Longitude = ptr(0), ptr(0) }},
		{
			name:     "offset timestamp normalized to UTC",
			mutate:   func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15T16:05:00+05:30" },
			wantTime: now,
		},
		{
			name:     "fractional seconds accepted",
			mutate:   func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15T10:34:59.123Z" },
			wantTime: time.Date(2025, 1, 15, 10, 34, 59, 123e6, time.UTC),
		},
		{name: "4m59s in the future allowed", mutate: func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15T10:39:59Z" }},
		{name: "very old timestamp allowed", mutate: func(in *UpdateLocationInput) { in.Timestamp = "2001-01-01T00:00:00Z" }},
		{
			name:       "everything missing",
			mutate:     func(in *UpdateLocationInput) { *in = UpdateLocationInput{VehicleID: "  "} },
			wantFields: map[string]string{"vehicle_id": "is required", "latitude": "is required", "longitude": "is required", "timestamp": "is required"},
		},
		{
			name:       "out of range",
			mutate:     func(in *UpdateLocationInput) { in.Latitude, in.Longitude = ptr(90.0001), ptr(-180.5) },
			wantFields: map[string]string{"latitude": "must be between -90 and 90", "longitude": "must be between -180 and 180"},
		},
		{
			name:       "not RFC 3339",
			mutate:     func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15 10:35:00" },
			wantFields: map[string]string{"timestamp": "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-15T10:35:00Z"},
		},
		{
			name:       "missing timezone",
			mutate:     func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15T10:35:00" },
			wantFields: map[string]string{"timestamp": "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-15T10:35:00Z"},
		},
		{
			name:       "more than 5 minutes in the future",
			mutate:     func(in *UpdateLocationInput) { in.Timestamp = "2025-01-15T10:40:01Z" },
			wantFields: map[string]string{"timestamp": "must not be more than 5m0s in the future"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := valid()
			tt.mutate(&in)
			u, err := ValidateLocationUpdate(in, now)
			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Location.Lat != *in.Latitude || u.Location.Lng != *in.Longitude {
				t.Errorf("location = %+v", u.Location)
			}
			if !tt.wantTime.IsZero() && (!u.RecordedAt.Equal(tt.wantTime) || u.RecordedAt.Location() != time.UTC) {
				t.Errorf("recorded_at = %v, want %v in UTC", u.RecordedAt, tt.wantTime)
			}
		})
	}
}

// --- service flow with an in-memory transaction ---

type fakeTx struct {
	inserts    int
	vehicle    model.VehicleLocation
	lockErr    error
	prev, curr []model.GeofenceRef
	events     []model.GeofenceEvent
	applied    bool
	failApply  bool
}

func (f *fakeTx) LockVehicle(context.Context, string) (model.VehicleLocation, error) {
	return f.vehicle, f.lockErr
}
func (f *fakeTx) InsertLocation(context.Context, LocationUpdate) error {
	f.inserts++
	return nil
}
func (f *fakeTx) CurrentGeofences(context.Context, string) ([]model.GeofenceRef, error) {
	return f.prev, nil
}
func (f *fakeTx) ContainingGeofences(context.Context, model.LatLng) ([]model.GeofenceRef, error) {
	return f.curr, nil
}
func (f *fakeTx) InsertEvents(_ context.Context, e []model.GeofenceEvent) error {
	f.events = e
	return nil
}
func (f *fakeTx) ApplyState(context.Context, LocationUpdate, []model.GeofenceRef, []model.GeofenceRef) error {
	if f.failApply {
		return errors.New("disk full")
	}
	f.applied = true
	return nil
}

type fakeLocationRepo struct {
	tx        *fakeTx
	committed bool
}

func (r *fakeLocationRepo) InTx(_ context.Context, fn func(LocationTx) error) error {
	if err := fn(r.tx); err != nil {
		return err // rolled back
	}
	r.committed = true
	return nil
}

func (r *fakeLocationRepo) GetVehicleLocation(context.Context, string) (model.VehicleLocation, error) {
	return r.tx.vehicle, nil
}

type recordingDispatcher struct {
	repo              *fakeLocationRepo
	calls             int
	events            []model.GeofenceEvent
	committedAtLatest bool
}

func (d *recordingDispatcher) Dispatch(events []model.GeofenceEvent) {
	d.calls++
	d.events = events
	d.committedAtLatest = d.repo.committed
}

func TestUpdateLocationFlow(t *testing.T) {
	last := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	vehicle := model.VehicleLocation{
		Vehicle:      model.Vehicle{ID: "veh_1", VehicleNumber: "KA-01", DriverName: "John"},
		LastLocation: &model.LocationFix{RecordedAt: last},
	}
	input := func(ts string) UpdateLocationInput {
		return UpdateLocationInput{VehicleID: "veh_1", Latitude: ptr(1), Longitude: ptr(2), Timestamp: ts}
	}

	t.Run("transition: events built, dispatched after commit", func(t *testing.T) {
		tx := &fakeTx{vehicle: vehicle, prev: refs("A"), curr: refs("B")}
		repo := &fakeLocationRepo{tx: tx}
		d := &recordingDispatcher{repo: repo}
		res, err := NewLocationService(repo, d).UpdateLocation(context.Background(), input("2025-01-15T10:05:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		if d.calls != 1 || !d.committedAtLatest {
			t.Fatalf("dispatch calls=%d committedAtDispatch=%v, want 1 call after commit", d.calls, d.committedAtLatest)
		}
		if len(res.Events) != 2 || res.Events[0].EventType != model.EventExit || res.Events[0].Geofence.ID != "A" ||
			res.Events[1].EventType != model.EventEntry || res.Events[1].Geofence.ID != "B" {
			t.Fatalf("events = %+v, want exit A then entry B", res.Events)
		}
		e := res.Events[1]
		if e.VehicleNumber != "KA-01" || e.DriverName != "John" || e.Location != (model.LatLng{Lat: 1, Lng: 2}) ||
			!e.OccurredAt.Equal(last.Add(5*time.Minute)) || e.ID[:5] != "viol_" {
			t.Errorf("event fields wrong: %+v", e)
		}
		if !tx.applied || !slices.Equal(ids(res.CurrentGeofences), []string{"B"}) {
			t.Errorf("state not applied or wrong current set: %v", ids(res.CurrentGeofences))
		}
	})

	t.Run("no transition: nothing dispatched", func(t *testing.T) {
		tx := &fakeTx{vehicle: vehicle, prev: refs("A"), curr: refs("A")}
		repo := &fakeLocationRepo{tx: tx}
		d := &recordingDispatcher{repo: repo}
		if _, err := NewLocationService(repo, d).UpdateLocation(context.Background(), input("2025-01-15T10:05:00Z")); err != nil {
			t.Fatal(err)
		}
		if d.calls != 0 {
			t.Errorf("dispatched %d times, want 0", d.calls)
		}
	})

	t.Run("out of order: stored, no events, state untouched", func(t *testing.T) {
		tx := &fakeTx{vehicle: vehicle, prev: refs("A"), curr: refs("B")}
		repo := &fakeLocationRepo{tx: tx}
		d := &recordingDispatcher{repo: repo}
		res, err := NewLocationService(repo, d).UpdateLocation(context.Background(), input("2025-01-15T09:59:59Z"))
		if err != nil {
			t.Fatal(err)
		}
		if !res.Stale || len(res.Events) != 0 || tx.applied || d.calls != 0 || !repo.committed {
			t.Errorf("stale=%v events=%d applied=%v dispatched=%d committed=%v", res.Stale, len(res.Events), tx.applied, d.calls, repo.committed)
		}
		if !slices.Equal(ids(res.CurrentGeofences), []string{"A"}) {
			t.Errorf("current = %v, want the stored set [A]", ids(res.CurrentGeofences))
		}
	})

	t.Run("exact duplicate of the last report: idempotent no-op", func(t *testing.T) {
		dupVehicle := vehicle
		dupVehicle.LastLocation = &model.LocationFix{LatLng: model.LatLng{Lat: 1, Lng: 2}, RecordedAt: last}
		tx := &fakeTx{vehicle: dupVehicle, prev: refs("A"), curr: refs("B")}
		repo := &fakeLocationRepo{tx: tx}
		d := &recordingDispatcher{repo: repo}
		res, err := NewLocationService(repo, d).UpdateLocation(context.Background(), input("2025-01-15T10:00:00Z"))
		if err != nil {
			t.Fatal(err)
		}
		if !res.Duplicate || tx.inserts != 0 || tx.applied || len(res.Events) != 0 || d.calls != 0 {
			t.Errorf("duplicate=%v inserts=%d applied=%v events=%d dispatched=%d", res.Duplicate, tx.inserts, tx.applied, len(res.Events), d.calls)
		}
		if !slices.Equal(ids(res.CurrentGeofences), []string{"A"}) {
			t.Errorf("current = %v, want stored set", ids(res.CurrentGeofences))
		}
	})

	t.Run("same timestamp, different position: processed", func(t *testing.T) {
		v := vehicle
		v.LastLocation = &model.LocationFix{LatLng: model.LatLng{Lat: 9, Lng: 9}, RecordedAt: last}
		tx := &fakeTx{vehicle: v, prev: nil, curr: refs("A")}
		repo := &fakeLocationRepo{tx: tx}
		res, err := NewLocationService(repo, &recordingDispatcher{repo: repo}).UpdateLocation(context.Background(), input("2025-01-15T10:00:00Z"))
		if err != nil || res.Duplicate || tx.inserts != 1 || len(res.Events) != 1 {
			t.Errorf("res=%+v err=%v inserts=%d", res, err, tx.inserts)
		}
	})

	t.Run("rollback: nothing dispatched", func(t *testing.T) {
		tx := &fakeTx{vehicle: vehicle, prev: nil, curr: refs("A"), failApply: true}
		repo := &fakeLocationRepo{tx: tx}
		d := &recordingDispatcher{repo: repo}
		if _, err := NewLocationService(repo, d).UpdateLocation(context.Background(), input("2025-01-15T10:05:00Z")); err == nil {
			t.Fatal("expected error")
		}
		if d.calls != 0 || repo.committed {
			t.Errorf("dispatched=%d committed=%v after failed tx", d.calls, repo.committed)
		}
	})

	t.Run("unknown vehicle: not found propagates", func(t *testing.T) {
		tx := &fakeTx{lockErr: model.NewError(model.ErrNotFound, `vehicle "veh_x" not found`)}
		repo := &fakeLocationRepo{tx: tx}
		_, err := NewLocationService(repo, &recordingDispatcher{repo: repo}).UpdateLocation(context.Background(), input("2025-01-15T10:05:00Z"))
		if !errors.Is(err, model.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("first report ever: no stale check", func(t *testing.T) {
		tx := &fakeTx{vehicle: model.VehicleLocation{Vehicle: vehicle.Vehicle}, curr: refs("A")}
		repo := &fakeLocationRepo{tx: tx}
		res, err := NewLocationService(repo, &recordingDispatcher{repo: repo}).UpdateLocation(context.Background(), input("2001-01-01T00:00:00Z"))
		if err != nil || res.Stale || len(res.Events) != 1 {
			t.Errorf("res=%+v err=%v, want one entry event", res, err)
		}
	})
}
