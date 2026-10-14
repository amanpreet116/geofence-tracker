package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

var numericString = regexp.MustCompile(`^[0-9]+$`)

// testRouter builds the real router, filling unset handlers with fakes.
func testRouter(h Handlers) *Router {
	if h.Health == nil {
		h.Health = NewHealthHandler(fakePinger{})
	}
	if h.Geofences == nil {
		h.Geofences = NewGeofenceHandler(&fakeGeofenceService{})
	}
	if h.Vehicles == nil {
		h.Vehicles = NewVehicleHandler(&fakeVehicleService{})
	}
	if h.History == nil {
		h.History = NewViolationHandler(&fakeViolationService{})
	}
	if h.WS == nil {
		h.WS = http.NotFoundHandler()
	}
	if h.Alerts == nil {
		h.Alerts = NewAlertHandler(&fakeAlertService{})
	}
	if h.Locations == nil {
		h.Locations = NewLocationHandler(&fakeLocationService{})
	}
	return NewRouter(h)
}

func TestRouter(t *testing.T) {
	tests := []struct {
		name       string
		pingErr    error
		method     string
		path       string
		wantStatus int
		wantFields map[string]string
		wantAllow  string
	}{
		{
			name: "health ok", method: http.MethodGet, path: "/health",
			wantStatus: http.StatusOK,
			wantFields: map[string]string{"status": "ok", "db": "ok"},
		},
		{
			name: "health db down", pingErr: errors.New("boom"), method: http.MethodGet, path: "/health",
			wantStatus: http.StatusServiceUnavailable,
			wantFields: map[string]string{"status": "degraded", "db": "unavailable"},
		},
		{
			name: "unknown path", method: http.MethodGet, path: "/nope",
			wantStatus: http.StatusNotFound,
			wantFields: map[string]string{"error": "resource not found"},
		},
		{
			name: "wrong method", method: http.MethodDelete, path: "/health",
			wantStatus: http.StatusMethodNotAllowed,
			wantFields: map[string]string{"error": "method not allowed"},
			wantAllow:  "GET, HEAD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := testRouter(Handlers{Health: NewHealthHandler(fakePinger{err: tt.pingErr})})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q", ct)
			}
			if tt.wantAllow != "" && rec.Header().Get("Allow") != tt.wantAllow {
				t.Errorf("Allow = %q, want %q", rec.Header().Get("Allow"), tt.wantAllow)
			}

			var body map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
			}
			if ns, _ := body["time_ns"].(string); !numericString.MatchString(ns) {
				t.Errorf("time_ns = %#v, want numeric string", body["time_ns"])
			}
			for k, want := range tt.wantFields {
				if got := body[k]; got != want {
					t.Errorf("%s = %v, want %q", k, got, want)
				}
			}
		})
	}
}
