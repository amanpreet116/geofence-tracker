package model

import "time"

// Category classifies a geofence.
type Category string

const (
	CategoryDeliveryZone   Category = "delivery_zone"
	CategoryRestrictedZone Category = "restricted_zone"
	CategoryTollZone       Category = "toll_zone"
	CategoryCustomerArea   Category = "customer_area"
)

// Categories lists every valid category, in documentation order.
var Categories = []Category{CategoryDeliveryZone, CategoryRestrictedZone, CategoryTollZone, CategoryCustomerArea}

// Valid reports whether c is one of Categories.
func (c Category) Valid() bool {
	switch c {
	case CategoryDeliveryZone, CategoryRestrictedZone, CategoryTollZone, CategoryCustomerArea:
		return true
	}
	return false
}

const GeofenceStatusActive = "active"

// LatLng is a point in API order. Only the repository converts it to
// PostGIS (longitude, latitude) order.
type LatLng struct {
	Lat float64
	Lng float64
}

// Geofence is a closed polygon boundary. Coordinates is the exterior ring,
// first point repeated as the last.
type Geofence struct {
	ID          string
	Name        string
	Description string
	Category    Category
	Coordinates []LatLng
	Status      string
	CreatedAt   time.Time
}
