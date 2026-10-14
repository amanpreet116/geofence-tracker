package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

const (
	pgForeignKeyViolation = "23503"

	fkAlertGeofence   = "alert_rules_geofence_id_fkey"
	fkAlertVehicle    = "alert_rules_vehicle_id_fkey"
	uqAlertActiveRule = "alert_rules_active_unique"
)

type AlertRepository struct {
	pool *pgxpool.Pool
}

func NewAlertRepository(pool *pgxpool.Pool) *AlertRepository {
	return &AlertRepository{pool: pool}
}

var _ service.AlertRepository = (*AlertRepository)(nil)

// CreateRule inserts rule. Existence and uniqueness are enforced by the
// database in the same statement (no check-then-insert race): a foreign key
// violation becomes ErrNotFound naming the missing entity, and the partial
// unique index on active rules becomes ErrConflict.
func (r *AlertRepository) CreateRule(ctx context.Context, rule *model.AlertRule) error {
	err := r.pool.QueryRow(ctx, `
		INSERT INTO alert_rules (id, geofence_id, vehicle_id, event_type, status)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at`,
		rule.ID, rule.GeofenceID, rule.VehicleID, string(rule.EventType), rule.Status,
	).Scan(&rule.CreatedAt)
	if err != nil {
		return r.ruleWriteError(ctx, err, *rule)
	}
	rule.CreatedAt = rule.CreatedAt.UTC()
	return nil
}

func (r *AlertRepository) ruleWriteError(ctx context.Context, err error, rule model.AlertRule) error {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return fmt.Errorf("write alert rule: %w", err)
	}
	switch {
	case pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == fkAlertGeofence:
		return model.NewError(model.ErrNotFound, fmt.Sprintf("geofence %q not found", rule.GeofenceID))
	case pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == fkAlertVehicle:
		return model.NewError(model.ErrNotFound, fmt.Sprintf("vehicle %q not found", *rule.VehicleID))
	case pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == uqAlertActiveRule:
		e := model.NewError(model.ErrConflict, "an identical active alert rule already exists")
		if existing, lookupErr := r.activeDuplicateID(ctx, rule); lookupErr == nil {
			e.Details = map[string]string{"existing_alert_id": existing}
		}
		return e
	}
	return fmt.Errorf("write alert rule: %w", err)
}

func (r *AlertRepository) activeDuplicateID(ctx context.Context, rule model.AlertRule) (string, error) {
	var id string
	err := r.pool.QueryRow(ctx, `
		SELECT id FROM alert_rules
		WHERE status = 'active' AND id <> $1 AND geofence_id = $2
		  AND vehicle_id IS NOT DISTINCT FROM $3 AND event_type = $4`,
		rule.ID, rule.GeofenceID, rule.VehicleID, string(rule.EventType)).Scan(&id)
	return id, err
}

const ruleSelect = `
	SELECT r.id, r.geofence_id, g.name, r.vehicle_id, v.vehicle_number, r.event_type, r.status, r.created_at
	FROM alert_rules r
	JOIN geofences g ON g.id = r.geofence_id
	LEFT JOIN vehicles v ON v.id = r.vehicle_id`

func (r *AlertRepository) ListRules(ctx context.Context, f service.AlertFilter) ([]model.AlertRule, error) {
	w := &where{}
	if f.GeofenceID != "" {
		w.add("r.geofence_id = %s", f.GeofenceID)
	}
	if f.VehicleID != "" {
		w.add("r.vehicle_id = %s", f.VehicleID)
	}
	return r.queryRules(ctx, ruleSelect+w.sql()+` ORDER BY r.created_at DESC, r.id DESC`, w.args...)
}

func (r *AlertRepository) getRule(ctx context.Context, id string) (model.AlertRule, error) {
	rules, err := r.queryRules(ctx, ruleSelect+` WHERE r.id = $1`, id)
	if err != nil {
		return model.AlertRule{}, err
	}
	if len(rules) == 0 {
		return model.AlertRule{}, alertNotFound(id)
	}
	return rules[0], nil
}

func (r *AlertRepository) SetRuleStatus(ctx context.Context, id, status string) (model.AlertRule, error) {
	var rule model.AlertRule
	err := r.pool.QueryRow(ctx, `
		UPDATE alert_rules SET status = $2 WHERE id = $1
		RETURNING id, geofence_id, vehicle_id, event_type`, id, status,
	).Scan(&rule.ID, &rule.GeofenceID, &rule.VehicleID, &rule.EventType)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.AlertRule{}, alertNotFound(id)
	}
	if err != nil {
		return model.AlertRule{}, r.ruleWriteError(ctx, err, rule) // reactivating a duplicate → 409
	}
	return r.getRule(ctx, id)
}

func (r *AlertRepository) DeleteRule(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete alert rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return alertNotFound(id)
	}
	return nil
}

// CandidateRules fetches every active rule on any event's geofence that
// targets all vehicles or any event's vehicle — one indexed query for the
// whole batch. It may include rules for a (geofence, vehicle) pair that did
// not occur together; service.MatchRules makes the exact decision.
func (r *AlertRepository) CandidateRules(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertRule, error) {
	geofenceIDs := make([]string, 0, len(events))
	vehicleIDs := make([]string, 0, len(events))
	for _, e := range events {
		geofenceIDs = append(geofenceIDs, e.Geofence.ID)
		vehicleIDs = append(vehicleIDs, e.VehicleID)
	}
	return r.queryRules(ctx, ruleSelect+`
		WHERE r.status = 'active'
		  AND r.geofence_id = ANY($1::text[])
		  AND (r.vehicle_id IS NULL OR r.vehicle_id = ANY($2::text[]))
		ORDER BY r.created_at, r.id`, geofenceIDs, vehicleIDs)
}

func (r *AlertRepository) queryRules(ctx context.Context, query string, args ...any) ([]model.AlertRule, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	rules, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.AlertRule, error) {
		var a model.AlertRule
		err := row.Scan(&a.ID, &a.GeofenceID, &a.GeofenceName, &a.VehicleID, &a.VehicleNumber, &a.EventType, &a.Status, &a.CreatedAt)
		a.CreatedAt = a.CreatedAt.UTC()
		return a, err
	})
	if err != nil {
		return nil, fmt.Errorf("query alert rules: %w", err)
	}
	return rules, nil
}

func alertNotFound(id string) error {
	return model.NewError(model.ErrNotFound, fmt.Sprintf("alert %q not found", id))
}
