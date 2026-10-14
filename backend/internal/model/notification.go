package model

import "time"

// AlertPayload is the real-time alert message, EXACTLY as specified for
// /ws/alerts in docs/ASSIGNMENT.md. It is broadcast as-is and stored as the
// jsonb payload of alert_notifications, so the JSON tags are the contract.
type AlertPayload struct {
	EventID   string               `json:"event_id"`
	EventType string               `json:"event_type"`
	Timestamp string               `json:"timestamp"`
	Vehicle   AlertPayloadVehicle  `json:"vehicle"`
	Geofence  AlertPayloadGeofence `json:"geofence"`
	Location  AlertPayloadLocation `json:"location"`
}

type AlertPayloadVehicle struct {
	VehicleID     string `json:"vehicle_id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
}

type AlertPayloadGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type AlertPayloadLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// AlertNotification is one dispatched alert: a (geofence event, rule) pair.
type AlertNotification struct {
	ID          string // evt_..., equals Payload.EventID
	AlertRuleID string
	EventID     string // geofence_events.id (viol_...)
	Payload     AlertPayload
	CreatedAt   time.Time
}
