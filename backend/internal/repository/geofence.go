package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

type GeofenceRepository struct {
	pool *pgxpool.Pool
}

func NewGeofenceRepository(pool *pgxpool.Pool) *GeofenceRepository {
	return &GeofenceRepository{pool: pool}
}

// CheckPolygon reports whether ring is a valid polygon according to PostGIS.
// ring must already be closed with at least 4 points (ST_MakePolygon
// requirement); the service validates that first.
func (r *GeofenceRepository) CheckPolygon(ctx context.Context, ring []model.LatLng) (bool, string, error) {
	lngs, lats := splitRing(ring)
	query := fmt.Sprintf(`SELECT ST_IsValid(g), ST_IsValidReason(g) FROM (SELECT %s AS g) AS t`, polygonSQL("$1", "$2"))

	var (
		valid  bool
		reason string
	)
	if err := r.pool.QueryRow(ctx, query, lngs, lats).Scan(&valid, &reason); err != nil {
		return false, "", fmt.Errorf("check polygon: %w", err)
	}
	return valid, apiOrderReason(reason), nil
}

// CreateGeofence inserts g and fills in CreatedAt from the database.
func (r *GeofenceRepository) CreateGeofence(ctx context.Context, g *model.Geofence) error {
	lngs, lats := splitRing(g.Coordinates)
	query := fmt.Sprintf(`
		INSERT INTO geofences (id, name, description, category, status, boundary)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, %s)
		RETURNING created_at`, polygonSQL("$6", "$7"))

	err := r.pool.QueryRow(ctx, query, g.ID, g.Name, g.Description, string(g.Category), g.Status, lngs, lats).
		Scan(&g.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert geofence: %w", err)
	}
	return nil
}

const geofenceColumns = `g.id, g.name, COALESCE(g.description, ''), g.category, g.status, g.created_at, `

// GetGeofence returns one geofence or model.ErrNotFound.
func (r *GeofenceRepository) GetGeofence(ctx context.Context, id string) (model.Geofence, error) {
	query := `SELECT ` + geofenceColumns + ringPointsSQL("g.boundary") + ` FROM geofences g WHERE g.id = $1`
	rows, err := r.pool.Query(ctx, query, id)
	if err != nil {
		return model.Geofence{}, fmt.Errorf("get geofence: %w", err)
	}
	g, err := pgx.CollectExactlyOneRow(rows, scanGeofence)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Geofence{}, model.NewError(model.ErrNotFound, fmt.Sprintf("geofence %q not found", id))
	}
	if err != nil {
		return model.Geofence{}, fmt.Errorf("get geofence: %w", err)
	}
	return g, nil
}

// ListGeofences returns geofences newest first; an empty category means all.
func (r *GeofenceRepository) ListGeofences(ctx context.Context, category model.Category) ([]model.Geofence, error) {
	query := `SELECT ` + geofenceColumns + ringPointsSQL("g.boundary") + ` FROM geofences g`
	var args []any
	if category != "" {
		query += ` WHERE g.category = $1`
		args = append(args, string(category))
	}
	query += ` ORDER BY g.created_at DESC, g.id DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list geofences: %w", err)
	}
	geofences, err := pgx.CollectRows(rows, scanGeofence)
	if err != nil {
		return nil, fmt.Errorf("list geofences: %w", err)
	}
	return geofences, nil
}

func scanGeofence(row pgx.CollectableRow) (model.Geofence, error) {
	var (
		g         model.Geofence
		category  string
		createdAt time.Time
		xy        [][]float64
	)
	if err := row.Scan(&g.ID, &g.Name, &g.Description, &category, &g.Status, &createdAt, &xy); err != nil {
		return model.Geofence{}, err
	}
	ring, err := joinRing(xy)
	if err != nil {
		return model.Geofence{}, fmt.Errorf("geofence %s: %w", g.ID, err)
	}
	g.Category = model.Category(category)
	g.CreatedAt = createdAt.UTC()
	g.Coordinates = ring
	return g, nil
}
