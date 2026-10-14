package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

type ViolationRepository struct {
	pool *pgxpool.Pool
}

func NewViolationRepository(pool *pgxpool.Pool) *ViolationRepository {
	return &ViolationRepository{pool: pool}
}

var _ service.ViolationRepository = (*ViolationRepository)(nil)

// ListViolations returns one page plus total_count.
//
// total_count uses a separate COUNT(*) with the same WHERE rather than
// COUNT(*) OVER():
//   - the window function makes Postgres materialize every matching row
//     before LIMIT applies, defeating the top-N index scan the page query
//     gets from the (…, occurred_at DESC) indexes;
//   - with OFFSET past the end the windowed query returns no rows, so the
//     total would be lost (we'd report 0 instead of the real count).
//
// Both queries run in one pipelined batch inside a read-only REPEATABLE READ
// transaction, so the page and the count see the same snapshot.
func (r *ViolationRepository) ListViolations(ctx context.Context, f service.ViolationFilter) ([]model.GeofenceEvent, int, error) {
	w := &where{}
	if f.VehicleID != "" {
		w.add("e.vehicle_id = %s", f.VehicleID)
	}
	if f.GeofenceID != "" {
		w.add("e.geofence_id = %s", f.GeofenceID)
	}
	if f.Start != nil {
		w.add("e.occurred_at >= %s", *f.Start)
	}
	if f.End != nil {
		w.add("e.occurred_at <= %s", *f.End)
	}
	filterSQL := w.sql()
	countArgs := append([]any(nil), w.args...)

	// The count only needs geofence_events: FKs guarantee the joins match.
	countQuery := `SELECT count(*) FROM geofence_events e` + filterSQL
	pageQuery := `
		SELECT e.id, e.event_type, e.vehicle_id, v.vehicle_number, v.driver_name,
		       e.geofence_id, g.name, g.category, e.latitude, e.longitude, e.occurred_at
		FROM geofence_events e
		JOIN vehicles v ON v.id = e.vehicle_id
		JOIN geofences g ON g.id = e.geofence_id` + filterSQL + `
		ORDER BY e.occurred_at DESC, e.id DESC
		LIMIT ` + w.arg(f.Limit) + ` OFFSET ` + w.arg(f.Offset)

	var (
		page  []model.GeofenceEvent
		total int
	)
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		batch.Queue(countQuery, countArgs...).QueryRow(func(row pgx.Row) error {
			return row.Scan(&total)
		})
		batch.Queue(pageQuery, w.args...).Query(func(rows pgx.Rows) error {
			var err error
			page, err = pgx.CollectRows(rows, scanViolation)
			return err
		})
		return tx.SendBatch(ctx, batch).Close()
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list violations: %w", err)
	}
	return page, total, nil
}

// latitude/longitude are plain columns stored in API order alongside the
// PostGIS point, so no conversion is needed here.
func scanViolation(row pgx.CollectableRow) (model.GeofenceEvent, error) {
	var e model.GeofenceEvent
	err := row.Scan(&e.ID, &e.EventType, &e.VehicleID, &e.VehicleNumber, &e.DriverName,
		&e.Geofence.ID, &e.Geofence.Name, &e.Geofence.Category, &e.Location.Lat, &e.Location.Lng, &e.OccurredAt)
	e.OccurredAt = e.OccurredAt.UTC()
	return e, err
}
