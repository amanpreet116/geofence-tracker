package model

import "time"

const VehicleStatusActive = "active"

// Vehicle is a registered vehicle.
type Vehicle struct {
	ID            string
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
	Status        string
	CreatedAt     time.Time
}

// LocationFix is a position reported at a point in time.
type LocationFix struct {
	LatLng
	RecordedAt time.Time
}

// FleetVehicle is a vehicle with its last known position (nil if it has
// never reported) and the geofences it is currently inside.
type FleetVehicle struct {
	Vehicle
	LastLocation       *LocationFix
	CurrentGeofenceIDs []string
}
