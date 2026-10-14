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
)

const pgUniqueViolation = "23505"

type VehicleRepository struct {
	pool *pgxpool.Pool
}

func NewVehicleRepository(pool *pgxpool.Pool) *VehicleRepository {
	return &VehicleRepository{pool: pool}
}

// CreateVehicle inserts v and fills in CreatedAt. A duplicate
// vehicle_number yields a model.ErrConflict error.
func (r *VehicleRepository) CreateVehicle(ctx context.Context, v *model.Vehicle) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`,
		v.ID, v.VehicleNumber, v.DriverName, v.VehicleType, v.Phone, v.Status,
	).Scan(&v.CreatedAt)

	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgUniqueViolation {
		return model.NewError(model.ErrConflict,
			fmt.Sprintf("vehicle_number %q is already registered", v.VehicleNumber))
	}
	if err != nil {
		return fmt.Errorf("insert vehicle: %w", err)
	}
	v.CreatedAt = v.CreatedAt.UTC()
	return nil
}

const vehicleColumns = `v.id, v.vehicle_number, v.driver_name, v.vehicle_type, v.phone, v.status, v.created_at`

// ListVehicles returns all vehicles, newest first.
func (r *VehicleRepository) ListVehicles(ctx context.Context) ([]model.Vehicle, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+vehicleColumns+` FROM vehicles v ORDER BY v.created_at DESC, v.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	vehicles, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Vehicle, error) {
		var v model.Vehicle
		err := row.Scan(&v.ID, &v.VehicleNumber, &v.DriverName, &v.VehicleType, &v.Phone, &v.Status, &v.CreatedAt)
		v.CreatedAt = v.CreatedAt.UTC()
		return v, err
	})
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	return vehicles, nil
}

// ListFleetLocations returns every vehicle with its last known position and
// the geofences it is currently inside, in a single query (no N+1).
// last_latitude/last_longitude are plain columns already in API order.
func (r *VehicleRepository) ListFleetLocations(ctx context.Context) ([]model.FleetVehicle, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+vehicleColumns+`,
		       v.last_latitude, v.last_longitude, v.last_location_at,
		       COALESCE(array_agg(s.geofence_id ORDER BY s.geofence_id)
		                FILTER (WHERE s.geofence_id IS NOT NULL), '{}') AS geofence_ids
		FROM vehicles v
		LEFT JOIN vehicle_geofence_state s ON s.vehicle_id = v.id
		GROUP BY v.id
		ORDER BY v.created_at DESC, v.id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list fleet locations: %w", err)
	}
	fleet, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.FleetVehicle, error) {
		var (
			f        model.FleetVehicle
			lat, lng *float64
			at       *time.Time
		)
		err := row.Scan(&f.ID, &f.VehicleNumber, &f.DriverName, &f.VehicleType, &f.Phone, &f.Status, &f.CreatedAt,
			&lat, &lng, &at, &f.CurrentGeofenceIDs)
		f.CreatedAt = f.CreatedAt.UTC()
		if lat != nil && lng != nil && at != nil {
			f.LastLocation = &model.LocationFix{LatLng: model.LatLng{Lat: *lat, Lng: *lng}, RecordedAt: at.UTC()}
		}
		return f, err
	})
	if err != nil {
		return nil, fmt.Errorf("list fleet locations: %w", err)
	}
	return fleet, nil
}
