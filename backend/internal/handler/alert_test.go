package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

type fakeAlertService struct {
	rules []model.AlertRule
}

func (f *fakeAlertService) Configure(_ context.Context, in service.ConfigureAlertInput) (model.AlertRule, error) {
	rule, err := service.ValidateConfigureAlert(in)
	if err != nil {
		return model.AlertRule{}, err
	}
	if rule.GeofenceID != "geo_123" {
		return model.AlertRule{}, model.NewError(model.ErrNotFound, `geofence "`+rule.GeofenceID+`" not found`)
	}
	rule.ID = "alert_789"
	return rule, nil
}

func (f *fakeAlertService) List(context.Context, service.AlertFilter) ([]model.AlertRule, error) {
	return f.rules, nil
}

func (f *fakeAlertService) SetStatus(_ context.Context, id, status string) (model.AlertRule, error) {
	s, err := service.ValidateAlertStatus(status)
	if err != nil {
		return model.AlertRule{}, err
	}
	for _, r := range f.rules {
		if r.ID == id {
			r.Status = s
			return r, nil
		}
	}
	return model.AlertRule{}, model.NewError(model.ErrNotFound, `alert "`+id+`" not found`)
}

func (f *fakeAlertService) Delete(_ context.Context, id string) error {
	for _, r := range f.rules {
		if r.ID == id {
			return nil
		}
	}
	return model.NewError(model.ErrNotFound, `alert "`+id+`" not found`)
}

func (f *fakeAlertService) RecentNotifications(_ context.Context, limit string) ([]model.AlertPayload, error) {
	if _, err := service.ParseNotificationLimit(limit); err != nil {
		return nil, err
	}
	return nil, nil
}

func alertRouter() *Router {
	veh, num := "veh_456", "KA-01-AB-1234"
	created := time.Date(2025, 1, 15, 9, 15, 0, 0, time.UTC)
	return testRouter(Handlers{Alerts: NewAlertHandler(&fakeAlertService{rules: []model.AlertRule{
		{ID: "alert_789", GeofenceID: "geo_123", GeofenceName: "Downtown Delivery Zone", VehicleID: &veh, VehicleNumber: &num,
			EventType: "entry", Status: "active", CreatedAt: created},
		{ID: "alert_all", GeofenceID: "geo_123", GeofenceName: "Downtown Delivery Zone",
			EventType: "both", Status: "active", CreatedAt: created},
	}})})
}

func TestAlertHandlers(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantBody   string // prefix up to time_ns
	}{
		{
			name: "configure specific vehicle", method: http.MethodPost, path: "/alerts/configure",
			body:       `{"geofence_id":"geo_123","vehicle_id":"veh_456","event_type":"entry"}`,
			wantStatus: http.StatusCreated,
			wantBody:   `{"alert_id":"alert_789","geofence_id":"geo_123","vehicle_id":"veh_456","event_type":"entry","status":"active","time_ns":"`,
		},
		{
			name: "configure all vehicles → vehicle_id null", method: http.MethodPost, path: "/alerts/configure",
			body:       `{"geofence_id":"geo_123","event_type":"both"}`,
			wantStatus: http.StatusCreated,
			wantBody:   `{"alert_id":"alert_789","geofence_id":"geo_123","vehicle_id":null,"event_type":"both","status":"active","time_ns":"`,
		},
		{
			name: "configure unknown geofence", method: http.MethodPost, path: "/alerts/configure",
			body:       `{"geofence_id":"geo_x","event_type":"entry"}`,
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"geofence \"geo_x\" not found","time_ns":"`,
		},
		{
			name: "configure bad event type", method: http.MethodPost, path: "/alerts/configure",
			body:       `{"geofence_id":"geo_123","event_type":"enter"}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid alert rule","details":{"event_type":"must be one of entry, exit, both"},"time_ns":"`,
		},
		{
			name: "list joins names; all-vehicles rule has nulls", method: http.MethodGet, path: "/alerts?geofence_id=geo_123",
			wantStatus: http.StatusOK,
			wantBody: `{"alerts":[` +
				`{"alert_id":"alert_789","geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","vehicle_id":"veh_456","vehicle_number":"KA-01-AB-1234","event_type":"entry","status":"active","created_at":"2025-01-15T09:15:00Z"},` +
				`{"alert_id":"alert_all","geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","vehicle_id":null,"vehicle_number":null,"event_type":"both","status":"active","created_at":"2025-01-15T09:15:00Z"}` +
				`],"time_ns":"`,
		},
		{
			name: "patch status", method: http.MethodPatch, path: "/alerts/alert_789",
			body:       `{"status":"inactive"}`,
			wantStatus: http.StatusOK,
			wantBody:   `{"alert_id":"alert_789","geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","vehicle_id":"veh_456","vehicle_number":"KA-01-AB-1234","event_type":"entry","status":"inactive","created_at":"2025-01-15T09:15:00Z","time_ns":"`,
		},
		{
			name: "patch invalid status", method: http.MethodPatch, path: "/alerts/alert_789",
			body: `{"status":"paused"}`, wantStatus: http.StatusBadRequest,
			wantBody: `{"error":"invalid alert update","details":{"status":"must be one of active, inactive"},"time_ns":"`,
		},
		{
			name: "patch missing", method: http.MethodPatch, path: "/alerts/alert_nope",
			body: `{"status":"active"}`, wantStatus: http.StatusNotFound,
			wantBody: `{"error":"alert \"alert_nope\" not found","time_ns":"`,
		},
		{
			name: "delete", method: http.MethodDelete, path: "/alerts/alert_789", wantStatus: http.StatusOK,
			wantBody: `{"alert_id":"alert_789","deleted":true,"time_ns":"`,
		},
		{
			name: "notifications empty is []", method: http.MethodGet, path: "/alerts/notifications",
			wantStatus: http.StatusOK, wantBody: `{"notifications":[],"time_ns":"`,
		},
		{
			name: "notifications bad limit", method: http.MethodGet, path: "/alerts/notifications?limit=500",
			wantStatus: http.StatusBadRequest,
			wantBody:   `{"error":"invalid query parameter","details":{"limit":"must be an integer between 1 and 200"},"time_ns":"`,
		},
		{
			name: "delete missing", method: http.MethodDelete, path: "/alerts/alert_nope", wantStatus: http.StatusNotFound,
			wantBody: `{"error":"alert \"alert_nope\" not found","time_ns":"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, _ := serve(t, alertRouter(), tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus || !strings.HasPrefix(rec.Body.String(), tt.wantBody) {
				t.Errorf("got %d %s\nwant %d %s…", rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
		})
	}
}
