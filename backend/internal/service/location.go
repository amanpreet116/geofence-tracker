package service

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// maxFutureSkew is how far ahead of server time a reported timestamp may be
// (tolerates device clock drift without letting a bogus future timestamp
// freeze the vehicle: later real updates would all look out of order).
const maxFutureSkew = 5 * time.Minute

// UpdateLocationInput is the raw POST /vehicles/location payload. Latitude
// and Longitude are pointers so "missing" differs from a legitimate 0.
type UpdateLocationInput struct {
	VehicleID string
	Latitude  *float64
	Longitude *float64
	Timestamp string
}

// LocationUpdate is a validated location report.
type LocationUpdate struct {
	VehicleID  string
	Location   model.LatLng
	RecordedAt time.Time
}

// ValidateLocationUpdate checks in against the API rules; now is the server
// clock. All field errors are reported together.
func ValidateLocationUpdate(in UpdateLocationInput, now time.Time) (LocationUpdate, error) {
	errs := fieldErrors{}
	var out LocationUpdate

	out.VehicleID = strings.TrimSpace(in.VehicleID)
	if out.VehicleID == "" {
		errs.add("vehicle_id", "is required")
	}

	switch {
	case in.Latitude == nil:
		errs.add("latitude", "is required")
	case *in.Latitude < -90 || *in.Latitude > 90:
		errs.add("latitude", "must be between -90 and 90")
	default:
		out.Location.Lat = *in.Latitude
	}
	switch {
	case in.Longitude == nil:
		errs.add("longitude", "is required")
	case *in.Longitude < -180 || *in.Longitude > 180:
		errs.add("longitude", "must be between -180 and 180")
	default:
		out.Location.Lng = *in.Longitude
	}

	ts := strings.TrimSpace(in.Timestamp)
	if ts == "" {
		errs.add("timestamp", "is required")
	} else if t, err := time.Parse(time.RFC3339, ts); err != nil {
		errs.add("timestamp", "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-15T10:35:00Z")
	} else if t.After(now.Add(maxFutureSkew)) {
		errs.add("timestamp", fmt.Sprintf("must not be more than %s in the future", maxFutureSkew))
	} else {
		out.RecordedAt = t.UTC()
	}

	if err := errs.err("invalid location update"); err != nil {
		return LocationUpdate{}, err
	}
	return out, nil
}

// ComputeTransitions diffs the geofences a vehicle was inside (prev) against
// those it is inside now (curr). Membership is by geofence ID; both results
// are sorted by ID so event order is deterministic.
func ComputeTransitions(prev, curr []model.GeofenceRef) (entered, exited []model.GeofenceRef) {
	inPrev := make(map[string]bool, len(prev))
	for _, g := range prev {
		inPrev[g.ID] = true
	}
	inCurr := make(map[string]bool, len(curr))
	for _, g := range curr {
		inCurr[g.ID] = true
		if !inPrev[g.ID] {
			entered = append(entered, g)
		}
	}
	for _, g := range prev {
		if !inCurr[g.ID] {
			exited = append(exited, g)
		}
	}
	byID := func(a, b model.GeofenceRef) int { return cmp.Compare(a.ID, b.ID) }
	slices.SortFunc(entered, byID)
	slices.SortFunc(exited, byID)
	return entered, exited
}

// EventDispatcher delivers committed geofence events to real-time clients.
// Dispatch is called after the transaction commits and MUST NOT block: the
// HTTP response never waits on alert delivery. Implementations queue the
// events and do matching/delivery on their own goroutines.
type EventDispatcher interface {
	Dispatch(events []model.GeofenceEvent)
}

// NoopDispatcher discards events (used until real-time delivery is wired).
type NoopDispatcher struct{}

func (NoopDispatcher) Dispatch([]model.GeofenceEvent) {}

// LocationTx is the work available inside the location-update transaction.
type LocationTx interface {
	// LockVehicle loads the vehicle with SELECT ... FOR UPDATE, serializing
	// concurrent updates for the same vehicle. Returns model.ErrNotFound.
	LockVehicle(ctx context.Context, vehicleID string) (model.VehicleLocation, error)
	InsertLocation(ctx context.Context, u LocationUpdate) error
	// CurrentGeofences is the stored "inside" set (vehicle_geofence_state).
	CurrentGeofences(ctx context.Context, vehicleID string) ([]model.GeofenceRef, error)
	// ContainingGeofences is the set of active geofences covering p.
	ContainingGeofences(ctx context.Context, p model.LatLng) ([]model.GeofenceRef, error)
	InsertEvents(ctx context.Context, events []model.GeofenceEvent) error
	// ApplyState records entered/exited membership and the vehicle's last position.
	ApplyState(ctx context.Context, u LocationUpdate, entered, exited []model.GeofenceRef) error
}

// LocationRepository is the persistence the location service needs.
type LocationRepository interface {
	// InTx runs fn in one transaction, committing iff fn returns nil.
	InTx(ctx context.Context, fn func(tx LocationTx) error) error
	GetVehicleLocation(ctx context.Context, vehicleID string) (model.VehicleLocation, error)
}

type LocationService struct {
	repo       LocationRepository
	dispatcher EventDispatcher
	now        func() time.Time
}

func NewLocationService(repo LocationRepository, dispatcher EventDispatcher) *LocationService {
	return &LocationService{repo: repo, dispatcher: dispatcher, now: time.Now}
}

// LocationResult is the outcome of UpdateLocation.
type LocationResult struct {
	VehicleID        string
	CurrentGeofences []model.GeofenceRef
	Events           []model.GeofenceEvent
	// Stale is true when the report was older than the vehicle's latest one
	// and was stored in history only.
	Stale bool
	// Duplicate is true when the report repeated the last one exactly and
	// was not stored again.
	Duplicate bool
}

// UpdateLocation stores a location report, detects geofence entry/exit
// against the previous state, records events, and hands them to the
// dispatcher after commit.
func (s *LocationService) UpdateLocation(ctx context.Context, in UpdateLocationInput) (LocationResult, error) {
	u, err := ValidateLocationUpdate(in, s.now())
	if err != nil {
		return LocationResult{}, err
	}

	result := LocationResult{VehicleID: u.VehicleID}
	var lastSeen time.Time

	err = s.repo.InTx(ctx, func(tx LocationTx) error {
		vehicle, err := tx.LockVehicle(ctx, u.VehicleID)
		if err != nil {
			return err
		}
		prev, err := tx.CurrentGeofences(ctx, u.VehicleID)
		if err != nil {
			return err
		}

		// Exact duplicate of the last applied report (same time and position),
		// e.g. a client retry: idempotent no-op, nothing stored twice.
		if last := vehicle.LastLocation; last != nil &&
			last.RecordedAt.Equal(u.RecordedAt) && last.LatLng == u.Location {
			result.Duplicate = true
			result.CurrentGeofences = prev
			return nil
		}

		if err := tx.InsertLocation(ctx, u); err != nil {
			return err
		}

		// Out-of-order report (older than what we already applied): keep it in
		// history but do not touch current state or emit events. Applying it
		// would "rewind" the vehicle and fabricate exit/entry pairs that never
		// happened in real time; re-deriving the past timeline would require
		// rewriting events already delivered as alerts. Equal timestamps are
		// processed normally (an idempotent retry yields no transitions).
		if last := vehicle.LastLocation; last != nil && u.RecordedAt.Before(last.RecordedAt) {
			result.Stale = true
			result.CurrentGeofences = prev
			lastSeen = last.RecordedAt
			return nil
		}

		curr, err := tx.ContainingGeofences(ctx, u.Location)
		if err != nil {
			return err
		}
		entered, exited := ComputeTransitions(prev, curr)
		result.Events = buildEvents(vehicle.Vehicle, u, entered, exited)

		if err := tx.InsertEvents(ctx, result.Events); err != nil {
			return err
		}
		if err := tx.ApplyState(ctx, u, entered, exited); err != nil {
			return err
		}
		result.CurrentGeofences = curr
		return nil
	})
	if err != nil {
		return LocationResult{}, fmt.Errorf("update location: %w", err)
	}

	if result.Stale {
		slog.InfoContext(ctx, "out-of-order location stored in history only",
			"vehicle_id", u.VehicleID,
			"timestamp", u.RecordedAt,
			"last_location_at", lastSeen,
		)
	}
	// After commit: alerts only ever describe persisted events.
	if len(result.Events) > 0 {
		s.dispatcher.Dispatch(result.Events)
	}
	return result, nil
}

// buildEvents creates exit events before entry events: moving from A to B
// at one instant reads as "left A, entered B".
func buildEvents(v model.Vehicle, u LocationUpdate, entered, exited []model.GeofenceRef) []model.GeofenceEvent {
	events := make([]model.GeofenceEvent, 0, len(entered)+len(exited))
	add := func(t model.EventType, g model.GeofenceRef) {
		events = append(events, model.GeofenceEvent{
			ID:            model.NewID(model.PrefixViolation),
			EventType:     t,
			VehicleID:     v.ID,
			VehicleNumber: v.VehicleNumber,
			DriverName:    v.DriverName,
			Geofence:      g,
			Location:      u.Location,
			OccurredAt:    u.RecordedAt,
		})
	}
	for _, g := range exited {
		add(model.EventExit, g)
	}
	for _, g := range entered {
		add(model.EventEntry, g)
	}
	return events
}

// GetVehicleLocation returns the vehicle's last position and current
// geofences; model.ErrNotFound if the vehicle does not exist.
func (s *LocationService) GetVehicleLocation(ctx context.Context, vehicleID string) (model.VehicleLocation, error) {
	vl, err := s.repo.GetVehicleLocation(ctx, strings.TrimSpace(vehicleID))
	if err != nil {
		return model.VehicleLocation{}, fmt.Errorf("get vehicle location: %w", err)
	}
	return vl, nil
}
