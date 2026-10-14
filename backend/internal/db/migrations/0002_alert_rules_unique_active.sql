-- At most one ACTIVE rule per (geofence, vehicle-or-all, event_type).
-- COALESCE makes "all vehicles" (NULL) compare equal to itself; inactive
-- rules are excluded so a rule can be recreated after deactivation.
CREATE UNIQUE INDEX alert_rules_active_unique
    ON alert_rules (geofence_id, COALESCE(vehicle_id, ''), event_type)
    WHERE status = 'active';

ALTER TABLE alert_rules
    ADD CONSTRAINT alert_rules_status_check CHECK (status IN ('active', 'inactive'));
