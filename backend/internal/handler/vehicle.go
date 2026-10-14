package handler

import (
	"context"
	"net/http"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// VehicleService is what the vehicle handlers need from the service layer.
type VehicleService interface {
	Create(ctx context.Context, in service.CreateVehicleInput) (model.Vehicle, error)
	List(ctx context.Context) ([]model.Vehicle, error)
	FleetLocations(ctx context.Context) ([]model.FleetVehicle, error)
}

type VehicleHandler struct {
	svc VehicleService
}

func NewVehicleHandler(svc VehicleService) *VehicleHandler {
	return &VehicleHandler{svc: svc}
}

type createVehicleRequest struct {
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
}

type createVehicleResponse struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	Status        string `json:"status"`
	respond.Timed
}

type vehicleItem struct {
	ID            string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName    string `json:"driver_name"`
	VehicleType   string `json:"vehicle_type"`
	Phone         string `json:"phone"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

type listVehiclesResponse struct {
	Vehicles []vehicleItem `json:"vehicles"`
	respond.Timed
}

// Create handles POST /vehicles.
func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createVehicleRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	v, err := h.svc.Create(r.Context(), service.CreateVehicleInput(req))
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusCreated, &createVehicleResponse{ID: v.ID, VehicleNumber: v.VehicleNumber, Status: v.Status})
}

// List handles GET /vehicles.
func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicles, err := h.svc.List(r.Context())
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	items := make([]vehicleItem, 0, len(vehicles)) // [] not null when empty
	for _, v := range vehicles {
		items = append(items, toVehicleItem(v))
	}
	respond.JSON(w, r, http.StatusOK, &listVehiclesResponse{Vehicles: items})
}

func toVehicleItem(v model.Vehicle) vehicleItem {
	return vehicleItem{
		ID:            v.ID,
		VehicleNumber: v.VehicleNumber,
		DriverName:    v.DriverName,
		VehicleType:   v.VehicleType,
		Phone:         v.Phone,
		Status:        v.Status,
		CreatedAt:     formatTime(v.CreatedAt),
	}
}

// --- GET /fleet/locations (extra endpoint, not in the assignment spec) ---

type locationBody struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp string  `json:"timestamp"`
}

type fleetItem struct {
	VehicleID          string        `json:"vehicle_id"`
	VehicleNumber      string        `json:"vehicle_number"`
	DriverName         string        `json:"driver_name"`
	VehicleType        string        `json:"vehicle_type"`
	Status             string        `json:"status"`
	CurrentLocation    *locationBody `json:"current_location"` // null if never reported
	CurrentGeofenceIDs []string      `json:"current_geofence_ids"`
}

type fleetResponse struct {
	Vehicles []fleetItem `json:"vehicles"`
	respond.Timed
}

// FleetLocations handles GET /fleet/locations: every vehicle's last known
// position and current geofence IDs in one request, so the frontend map
// does not call GET /vehicles/location/{id} once per vehicle.
func (h *VehicleHandler) FleetLocations(w http.ResponseWriter, r *http.Request) {
	fleet, err := h.svc.FleetLocations(r.Context())
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	items := make([]fleetItem, 0, len(fleet))
	for _, f := range fleet {
		item := fleetItem{
			VehicleID:          f.ID,
			VehicleNumber:      f.VehicleNumber,
			DriverName:         f.DriverName,
			VehicleType:        f.VehicleType,
			Status:             f.Status,
			CurrentGeofenceIDs: f.CurrentGeofenceIDs,
		}
		if item.CurrentGeofenceIDs == nil {
			item.CurrentGeofenceIDs = []string{}
		}
		if loc := f.LastLocation; loc != nil {
			item.CurrentLocation = &locationBody{Latitude: loc.Lat, Longitude: loc.Lng, Timestamp: formatTime(loc.RecordedAt)}
		}
		items = append(items, item)
	}
	respond.JSON(w, r, http.StatusOK, &fleetResponse{Vehicles: items})
}
