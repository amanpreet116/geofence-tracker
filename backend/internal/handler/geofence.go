package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// GeofenceService is what the geofence handlers need from the service layer.
type GeofenceService interface {
	Create(ctx context.Context, in service.CreateGeofenceInput) (model.Geofence, error)
	List(ctx context.Context, category string) ([]model.Geofence, error)
}

type GeofenceHandler struct {
	svc GeofenceService
}

func NewGeofenceHandler(svc GeofenceService) *GeofenceHandler {
	return &GeofenceHandler{svc: svc}
}

type createGeofenceRequest struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
}

type createGeofenceResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
	respond.Timed
}

type geofenceItem struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Coordinates [][2]float64 `json:"coordinates"`
	Category    string       `json:"category"`
	CreatedAt   string       `json:"created_at"`
}

type listGeofencesResponse struct {
	Geofences []geofenceItem `json:"geofences"`
	respond.Timed
}

// Create handles POST /geofences.
func (h *GeofenceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createGeofenceRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}

	g, err := h.svc.Create(r.Context(), service.CreateGeofenceInput{
		Name:        req.Name,
		Description: req.Description,
		Coordinates: req.Coordinates,
		Category:    req.Category,
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusCreated, &createGeofenceResponse{ID: g.ID, Name: g.Name, Status: g.Status})
}

// List handles GET /geofences?category=.
func (h *GeofenceHandler) List(w http.ResponseWriter, r *http.Request) {
	geofences, err := h.svc.List(r.Context(), r.URL.Query().Get("category"))
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	items := make([]geofenceItem, 0, len(geofences)) // [] not null when empty
	for _, g := range geofences {
		items = append(items, toGeofenceItem(g))
	}
	respond.JSON(w, r, http.StatusOK, &listGeofencesResponse{Geofences: items})
}

func toGeofenceItem(g model.Geofence) geofenceItem {
	coords := make([][2]float64, len(g.Coordinates))
	for i, p := range g.Coordinates {
		coords[i] = [2]float64{p.Lat, p.Lng}
	}
	return geofenceItem{
		ID:          g.ID,
		Name:        g.Name,
		Description: g.Description,
		Coordinates: coords,
		Category:    string(g.Category),
		CreatedAt:   formatTime(g.CreatedAt),
	}
}

// formatTime renders timestamps per the contract: RFC 3339 in UTC.
func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
