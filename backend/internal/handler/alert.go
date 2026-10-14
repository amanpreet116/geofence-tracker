package handler

import (
	"context"
	"net/http"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// AlertService is what the alert handlers need from the service layer.
type AlertService interface {
	Configure(ctx context.Context, in service.ConfigureAlertInput) (model.AlertRule, error)
	List(ctx context.Context, f service.AlertFilter) ([]model.AlertRule, error)
	SetStatus(ctx context.Context, id, status string) (model.AlertRule, error)
	Delete(ctx context.Context, id string) error
	RecentNotifications(ctx context.Context, limit string) ([]model.AlertPayload, error)
}

type AlertHandler struct {
	svc AlertService
}

func NewAlertHandler(svc AlertService) *AlertHandler {
	return &AlertHandler{svc: svc}
}

type configureAlertRequest struct {
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"`
	EventType  string  `json:"event_type"`
}

type configureAlertResponse struct {
	AlertID    string  `json:"alert_id"`
	GeofenceID string  `json:"geofence_id"`
	VehicleID  *string `json:"vehicle_id"` // null = all vehicles
	EventType  string  `json:"event_type"`
	Status     string  `json:"status"`
	respond.Timed
}

// alertItem is one rule in GET /alerts. For rules covering all vehicles,
// vehicle_id and vehicle_number are both null.
type alertItem struct {
	AlertID       string  `json:"alert_id"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	VehicleID     *string `json:"vehicle_id"`
	VehicleNumber *string `json:"vehicle_number"`
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type listAlertsResponse struct {
	Alerts []alertItem `json:"alerts"`
	respond.Timed
}

type alertResponse struct {
	alertItem
	respond.Timed
}

type updateAlertRequest struct {
	Status string `json:"status"`
}

type deleteAlertResponse struct {
	AlertID string `json:"alert_id"`
	Deleted bool   `json:"deleted"`
	respond.Timed
}

// Configure handles POST /alerts/configure.
func (h *AlertHandler) Configure(w http.ResponseWriter, r *http.Request) {
	var req configureAlertRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	rule, err := h.svc.Configure(r.Context(), service.ConfigureAlertInput(req))
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusCreated, &configureAlertResponse{
		AlertID:    rule.ID,
		GeofenceID: rule.GeofenceID,
		VehicleID:  rule.VehicleID,
		EventType:  string(rule.EventType),
		Status:     rule.Status,
	})
}

// List handles GET /alerts?geofence_id=&vehicle_id=.
func (h *AlertHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	rules, err := h.svc.List(r.Context(), service.AlertFilter{GeofenceID: q.Get("geofence_id"), VehicleID: q.Get("vehicle_id")})
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	items := make([]alertItem, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toAlertItem(rule))
	}
	respond.JSON(w, r, http.StatusOK, &listAlertsResponse{Alerts: items})
}

// Update handles PATCH /alerts/{alert_id} (extra endpoint).
func (h *AlertHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req updateAlertRequest
	if err := respond.Decode(w, r, &req); err != nil {
		respond.Error(w, r, err)
		return
	}
	rule, err := h.svc.SetStatus(r.Context(), r.PathValue("alert_id"), req.Status)
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, &alertResponse{alertItem: toAlertItem(rule)})
}

// Delete handles DELETE /alerts/{alert_id} (extra endpoint). It answers 200
// with a body rather than 204 because every response carries time_ns.
func (h *AlertHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("alert_id")
	if err := h.svc.Delete(r.Context(), id); err != nil {
		respond.Error(w, r, err)
		return
	}
	respond.JSON(w, r, http.StatusOK, &deleteAlertResponse{AlertID: id, Deleted: true})
}

func toAlertItem(rule model.AlertRule) alertItem {
	return alertItem{
		AlertID:       rule.ID,
		GeofenceID:    rule.GeofenceID,
		GeofenceName:  rule.GeofenceName,
		VehicleID:     rule.VehicleID,
		VehicleNumber: rule.VehicleNumber,
		EventType:     string(rule.EventType),
		Status:        rule.Status,
		CreatedAt:     formatTime(rule.CreatedAt),
	}
}

type notificationsResponse struct {
	// Each item is the exact /ws/alerts message that was broadcast.
	Notifications []model.AlertPayload `json:"notifications"`
	respond.Timed
}

// Notifications handles GET /alerts/notifications?limit= (extra endpoint):
// recent dispatched alerts, so the live feed survives a page reload.
func (h *AlertHandler) Notifications(w http.ResponseWriter, r *http.Request) {
	payloads, err := h.svc.RecentNotifications(r.Context(), r.URL.Query().Get("limit"))
	if err != nil {
		respond.Error(w, r, err)
		return
	}
	if payloads == nil {
		payloads = []model.AlertPayload{}
	}
	respond.JSON(w, r, http.StatusOK, &notificationsResponse{Notifications: payloads})
}
