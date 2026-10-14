package model

import "time"

// EventType is a geofence transition.
type EventType string

const (
	EventEntry EventType = "entry"
	EventExit  EventType = "exit"
)

// GeofenceRef identifies a geofence with the fields responses and alerts need.
type GeofenceRef struct {
	ID       string
	Name     string
	Category Category
}

// GeofenceEvent is a recorded entry/exit (a "violation" in the API). It
// carries denormalized vehicle and geofence fields so alert delivery needs
// no further queries.
type GeofenceEvent struct {
	ID            string // viol_...
	EventType     EventType
	VehicleID     string
	VehicleNumber string
	DriverName    string
	Geofence      GeofenceRef
	Location      LatLng
	OccurredAt    time.Time
}

// VehicleLocation is a vehicle's last known position (nil if it never
// reported) and the geofences it is currently inside.
type VehicleLocation struct {
	Vehicle
	LastLocation     *LocationFix
	CurrentGeofences []GeofenceRef
}
