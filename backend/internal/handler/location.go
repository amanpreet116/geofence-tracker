package handler

import (
	"context"
	"net/http"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// LocationService is what the location handlers need from the service layer.
type LocationService interface {
	UpdateLocation(ctx context.Context, in service.UpdateLocationInput) (service.LocationResult, error)
	GetVehicleLocation(ctx context.Context, vehicleID string) (model.VehicleLocation, error)
}

type LocationHandler struct {
	svc LocationService
}

func NewLocationHandler(svc LocationService) *LocationHandler {
	return &LocationHandler{svc: svc}
}

type updateLocationRequest struct {
	VehicleID string   `json:"vehicle_id"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
	Timestamp string   `json:"timestamp"`
}

type insideGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status"`
}

type updateLocationResponse struct {
	VehicleID        string           `json:"vehicle_id"`
	LocationUpdated  bool             `json:"location_updated"`
	CurrentGeofences []insideGeofence `json:"current_geofences"`
	respond.Timed
}

type geofenceMembership struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Category     string `json:"category"`
}

type vehicleLocationResponse struct {
	VehicleID        string               `json:"vehicle_id"`
	VehicleNumber    string               `json:"vehicle_number"`
	CurrentLocation  *locationBody        `json:"current_location"` // null until first report
	CurrentGeofences []geofenceMembership `json:"current_geofences"`
	respond.Timed
}

// Update handles POST /vehicles/location.
func (h *LocationHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateLocationRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	res, err := h.svc.UpdateLocation(r.Context(), service.UpdateLocationInput(req))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	inside := make([]insideGeofence, 0, len(res.CurrentGeofences))
	for _, g := range res.CurrentGeofences {
		inside = append(inside, insideGeofence{GeofenceID: g.ID, GeofenceName: g.Name, Status: "inside"})
	}
	respond.JSON(w, r, http.StatusOK, &updateLocationResponse{
		VehicleID:        res.VehicleID,
		LocationUpdated:  true, // also for out-of-order reports: stored in history
		CurrentGeofences: inside,
	})
}

// Get handles GET /vehicles/location/{vehicle_id}.
func (h *LocationHandler) Get(w http.ResponseWriter, r *http.Request) {
	vl, err := h.svc.GetVehicleLocation(r.Context(), r.PathValue("vehicle_id"))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	resp := &vehicleLocationResponse{
		VehicleID:        vl.ID,
		VehicleNumber:    vl.VehicleNumber,
		CurrentGeofences: make([]geofenceMembership, 0, len(vl.CurrentGeofences)),
	}
	if loc := vl.LastLocation; loc != nil {
		resp.CurrentLocation = &locationBody{Latitude: loc.Lat, Longitude: loc.Lng, Timestamp: formatTime(loc.RecordedAt)}
	}
	for _, g := range vl.CurrentGeofences {
		resp.CurrentGeofences = append(resp.CurrentGeofences,
			geofenceMembership{GeofenceID: g.ID, GeofenceName: g.Name, Category: string(g.Category)})
	}
	respond.JSON(w, r, http.StatusOK, resp)
}
