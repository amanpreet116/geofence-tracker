-- One notification per (event, rule): makes dispatch idempotent, so a retry
-- or a duplicate enqueue can never alert twice for the same pair.
CREATE UNIQUE INDEX alert_notifications_event_rule_unique
    ON alert_notifications (geofence_event_id, alert_rule_id);
