package handler

import (
	"context"
	"net/http"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// ViolationService is what the history handler needs from the service layer.
type ViolationService interface {
	History(ctx context.Context, in service.ViolationQueryInput) ([]model.GeofenceEvent, int, error)
}

type ViolationHandler struct {
	svc ViolationService
}

func NewViolationHandler(svc ViolationService) *ViolationHandler {
	return &ViolationHandler{svc: svc}
}

type violationItem struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicle_id"`
	VehicleNumber string  `json:"vehicle_number"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	EventType     string  `json:"event_type"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     string  `json:"timestamp"`
}

type violationHistoryResponse struct {
	Violations []violationItem `json:"violations"`
	TotalCount int             `json:"total_count"`
	respond.Timed
}

// History handles GET /violations/history?vehicle_id=&geofence_id=&start_date=&end_date=&limit=&offset=
// (offset is an additive pagination parameter).
func (h *ViolationHandler) History(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	events, total, err := h.svc.History(r.Context(), service.ViolationQueryInput{
		VehicleID:  q.Get("vehicle_id"),
		GeofenceID: q.Get("geofence_id"),
		StartDate:  q.Get("start_date"),
		EndDate:    q.Get("end_date"),
		Limit:      q.Get("limit"),
		Offset:     q.Get("offset"),
	})
	if err != nil {
		respond.Error(w, r, err)
		return
	}

	items := make([]violationItem, 0, len(events))
	for _, e := range events {
		items = append(items, violationItem{
			ID:            e.ID,
			VehicleID:     e.VehicleID,
			VehicleNumber: e.VehicleNumber,
			GeofenceID:    e.Geofence.ID,
			GeofenceName:  e.Geofence.Name,
			EventType:     string(e.EventType),
			Latitude:      e.Location.Lat,
			Longitude:     e.Location.Lng,
			Timestamp:     formatTime(e.OccurredAt),
		})
	}
	respond.JSON(w, r, http.StatusOK, &violationHistoryResponse{Violations: items, TotalCount: total})
}
