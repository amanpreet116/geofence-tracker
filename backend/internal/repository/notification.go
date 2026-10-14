package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

const fkNotificationRule = "alert_notifications_alert_rule_id_fkey"

type NotificationRepository struct {
	pool *pgxpool.Pool
}

func NewNotificationRepository(pool *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{pool: pool}
}

var _ service.NotificationStore = (*NotificationRepository)(nil)

// InsertNotification stores n; the unique (geofence_event_id, alert_rule_id)
// index turns a repeat into a no-op (inserted=false).
func (r *NotificationRepository) InsertNotification(ctx context.Context, n model.AlertNotification) (bool, error) {
	payload, err := json.Marshal(n.Payload)
	if err != nil {
		return false, fmt.Errorf("encode payload: %w", err)
	}
	var id string
	err = r.pool.QueryRow(ctx, `
		INSERT INTO alert_notifications (id, alert_rule_id, geofence_event_id, payload)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (geofence_event_id, alert_rule_id) DO NOTHING
		RETURNING id`, n.ID, n.AlertRuleID, n.EventID, payload).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	// The rule was deleted between matching and this insert: no alert.
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok &&
		pgErr.Code == pgForeignKeyViolation && pgErr.ConstraintName == fkNotificationRule {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("insert notification: %w", err)
	}
	return true, nil
}

// ListPayloads returns the most recent alert payloads, newest first. They
// are decoded into the typed struct (not passed through raw) because jsonb
// does not preserve key order; re-encoding restores the spec's field order.
func (r *NotificationRepository) ListPayloads(ctx context.Context, limit int) ([]model.AlertPayload, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT payload FROM alert_notifications ORDER BY created_at DESC, id DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	payloads, err := pgx.CollectRows(rows, pgx.RowTo[model.AlertPayload])
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return payloads, nil
}
