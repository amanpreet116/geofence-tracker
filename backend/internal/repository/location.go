package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// querier is satisfied by both *pgxpool.Pool and pgx.Tx, so the same query
// code runs inside or outside a transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// LocationRepository implements service.LocationRepository.
type LocationRepository struct {
	pool *pgxpool.Pool
}

func NewLocationRepository(pool *pgxpool.Pool) *LocationRepository {
	return &LocationRepository{pool: pool}
}

var _ service.LocationRepository = (*LocationRepository)(nil)

// InTx runs fn in a READ COMMITTED transaction. Per-vehicle serialization
// comes from LockVehicle's row lock, not the isolation level.
func (r *LocationRepository) InTx(ctx context.Context, fn func(tx service.LocationTx) error) error {
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		return fn(locationQueries{q: tx})
	})
}

// GetVehicleLocation reads a vehicle's last position and current geofences
// without locking.
func (r *LocationRepository) GetVehicleLocation(ctx context.Context, vehicleID string) (model.VehicleLocation, error) {
	q := locationQueries{q: r.pool}
	vl, err := q.loadVehicle(ctx, vehicleID, false)
	if err != nil {
		return model.VehicleLocation{}, err
	}
	if vl.CurrentGeofences, err = q.CurrentGeofences(ctx, vehicleID); err != nil {
		return model.VehicleLocation{}, err
	}
	return vl, nil
}

// locationQueries implements service.LocationTx.
type locationQueries struct {
	q querier
}

var _ service.LocationTx = locationQueries{}

func (l locationQueries) LockVehicle(ctx context.Context, vehicleID string) (model.VehicleLocation, error) {
	return l.loadVehicle(ctx, vehicleID, true)
}

func (l locationQueries) loadVehicle(ctx context.Context, vehicleID string, forUpdate bool) (model.VehicleLocation, error) {
	query := `SELECT ` + vehicleColumns + `, v.last_latitude, v.last_longitude, v.last_location_at
		FROM vehicles v WHERE v.id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var (
		vl       model.VehicleLocation
		lat, lng *float64
		at       *time.Time
	)
	err := l.q.QueryRow(ctx, query, vehicleID).Scan(
		&vl.ID, &vl.VehicleNumber, &vl.DriverName, &vl.VehicleType, &vl.Phone, &vl.Status, &vl.CreatedAt,
		&lat, &lng, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.VehicleLocation{}, model.NewError(model.ErrNotFound, fmt.Sprintf("vehicle %q not found", vehicleID))
	}
	if err != nil {
		return model.VehicleLocation{}, fmt.Errorf("load vehicle: %w", err)
	}
	vl.CreatedAt = vl.CreatedAt.UTC()
	if lat != nil && lng != nil && at != nil {
		vl.LastLocation = &model.LocationFix{LatLng: model.LatLng{Lat: *lat, Lng: *lng}, RecordedAt: at.UTC()}
	}
	return vl, nil
}

func (l locationQueries) InsertLocation(ctx context.Context, u service.LocationUpdate) error {
	lng, lat := pointArgs(u.Location)
	_, err := l.q.Exec(ctx,
		`INSERT INTO vehicle_locations (vehicle_id, location, recorded_at) VALUES ($1, `+pointSQL("$2", "$3")+`, $4)`,
		u.VehicleID, lng, lat, u.RecordedAt)
	if err != nil {
		return fmt.Errorf("insert location: %w", err)
	}
	return nil
}

func (l locationQueries) CurrentGeofences(ctx context.Context, vehicleID string) ([]model.GeofenceRef, error) {
	return l.geofenceRefs(ctx, `
		SELECT g.id, g.name, g.category
		FROM vehicle_geofence_state s
		JOIN geofences g ON g.id = s.geofence_id
		WHERE s.vehicle_id = $1
		ORDER BY g.name, g.id`, vehicleID)
}

// ContainingGeofences uses ST_Covers rather than ST_Contains/ST_Within so a
// point exactly ON the boundary counts as inside: a vehicle on the fence
// line of a restricted zone should alert, and it avoids flapping entry/exit
// for points that sit on an edge. ST_Covers is index-aware, so the GiST
// index on boundary prunes candidates by bounding box before the exact test.
// Geometry is planar in degrees (SRID 4326), which is accurate for
// containment of city-scale polygons that do not cross the antimeridian.
func (l locationQueries) ContainingGeofences(ctx context.Context, p model.LatLng) ([]model.GeofenceRef, error) {
	lng, lat := pointArgs(p)
	return l.geofenceRefs(ctx, `
		SELECT g.id, g.name, g.category
		FROM geofences g
		WHERE g.status = 'active' AND ST_Covers(g.boundary, `+pointSQL("$1", "$2")+`)
		ORDER BY g.name, g.id`, lng, lat)
}

func (l locationQueries) geofenceRefs(ctx context.Context, query string, args ...any) ([]model.GeofenceRef, error) {
	rows, err := l.q.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query geofences: %w", err)
	}
	refs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.GeofenceRef, error) {
		var g model.GeofenceRef
		err := row.Scan(&g.ID, &g.Name, &g.Category)
		return g, err
	})
	if err != nil {
		return nil, fmt.Errorf("query geofences: %w", err)
	}
	return refs, nil
}

func (l locationQueries) InsertEvents(ctx context.Context, events []model.GeofenceEvent) error {
	if len(events) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, e := range events {
		lng, lat := pointArgs(e.Location)
		batch.Queue(`
			INSERT INTO geofence_events (id, vehicle_id, geofence_id, event_type, location, latitude, longitude, occurred_at)
			VALUES ($1, $2, $3, $4, `+pointSQL("$5", "$6")+`, $6, $5, $7)`,
			e.ID, e.VehicleID, e.Geofence.ID, string(e.EventType), lng, lat, e.OccurredAt)
	}
	if err := l.q.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert geofence events: %w", err)
	}
	return nil
}

func (l locationQueries) ApplyState(ctx context.Context, u service.LocationUpdate, entered, exited []model.GeofenceRef) error {
	batch := &pgx.Batch{}
	if len(exited) > 0 {
		batch.Queue(`DELETE FROM vehicle_geofence_state WHERE vehicle_id = $1 AND geofence_id = ANY($2::text[])`,
			u.VehicleID, refIDs(exited))
	}
	if len(entered) > 0 {
		batch.Queue(`
			INSERT INTO vehicle_geofence_state (vehicle_id, geofence_id, entered_at)
			SELECT $1, gid, $3 FROM unnest($2::text[]) AS gid
			ON CONFLICT (vehicle_id, geofence_id) DO NOTHING`,
			u.VehicleID, refIDs(entered), u.RecordedAt)
	}
	// last_latitude/last_longitude are plain columns in API order.
	batch.Queue(`UPDATE vehicles SET last_latitude = $2, last_longitude = $3, last_location_at = $4 WHERE id = $1`,
		u.VehicleID, u.Location.Lat, u.Location.Lng, u.RecordedAt)

	if err := l.q.SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("apply geofence state: %w", err)
	}
	return nil
}

func refIDs(refs []model.GeofenceRef) []string {
	ids := make([]string, len(refs))
	for i, r := range refs {
		ids[i] = r.ID
	}
	return ids
}
