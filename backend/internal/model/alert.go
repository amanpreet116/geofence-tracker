package model

import "time"

// AlertEventType is what an alert rule fires on.
type AlertEventType string

const (
	AlertOnEntry AlertEventType = "entry"
	AlertOnExit  AlertEventType = "exit"
	AlertOnBoth  AlertEventType = "both"
)

// Valid reports whether t is entry, exit or both.
func (t AlertEventType) Valid() bool {
	return t == AlertOnEntry || t == AlertOnExit || t == AlertOnBoth
}

// Alert rule statuses.
const (
	AlertStatusActive   = "active"
	AlertStatusInactive = "inactive"
)

// AlertRule fires on geofence events. VehicleID nil means "all vehicles";
// VehicleNumber is then nil as well.
type AlertRule struct {
	ID            string
	GeofenceID    string
	GeofenceName  string
	VehicleID     *string
	VehicleNumber *string
	EventType     AlertEventType
	Status        string
	CreatedAt     time.Time
}

// AlertMatch is an event together with every rule it triggers.
type AlertMatch struct {
	Event GeofenceEvent
	Rules []AlertRule
}
