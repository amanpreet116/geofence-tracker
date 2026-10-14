package handler

import (
	"net/http"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
)

// Router wraps http.ServeMux so that unmatched paths (404) and wrong methods
// (405) return the contract JSON error body instead of ServeMux's plain text.
type Router struct {
	mux *http.ServeMux
}

// Handlers groups every handler the router serves.
type Handlers struct {
	Health    *HealthHandler
	Geofences *GeofenceHandler
	Vehicles  *VehicleHandler
	Locations *LocationHandler
	Alerts    *AlertHandler
	History   *ViolationHandler
	WS        http.Handler // GET /ws/alerts (realtime.Hub.ServeWS)
}

// NewRouter registers all routes. Paths and methods are fixed by the API
// contract in docs/ASSIGNMENT.md.
func NewRouter(h Handlers) *Router {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health.Get)
	mux.HandleFunc("POST /geofences", h.Geofences.Create)
	mux.HandleFunc("GET /geofences", h.Geofences.List)
	mux.HandleFunc("POST /vehicles", h.Vehicles.Create)
	mux.HandleFunc("GET /vehicles", h.Vehicles.List)
	mux.HandleFunc("POST /vehicles/location", h.Locations.Update)
	mux.HandleFunc("GET /vehicles/location/{vehicle_id}", h.Locations.Get)
	mux.HandleFunc("POST /alerts/configure", h.Alerts.Configure)
	mux.HandleFunc("GET /alerts", h.Alerts.List)
	mux.HandleFunc("GET /violations/history", h.History.History)
	mux.Handle("GET /ws/alerts", h.WS)

	// Extra endpoints (not in the assignment spec); see docs/EXTRA_ENDPOINTS.md.
	mux.HandleFunc("GET /fleet/locations", h.Vehicles.FleetLocations)
	mux.HandleFunc("PATCH /alerts/{alert_id}", h.Alerts.Update)
	mux.HandleFunc("DELETE /alerts/{alert_id}", h.Alerts.Delete)
	mux.HandleFunc("GET /alerts/notifications", h.Alerts.Notifications)
	return &Router{mux: mux}
}

func (rt *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, pattern := rt.mux.Handler(r); pattern != "" {
		rt.mux.ServeHTTP(w, r)
		return
	}

	// Let ServeMux decide between 404 and 405 (and compute Allow), but
	// capture its plain-text output and replace it with JSON.
	capture := &headerCapture{header: http.Header{}}
	rt.mux.ServeHTTP(capture, r)

	if capture.status == http.StatusMethodNotAllowed {
		MethodNotAllowed(w, r, capture.header.Get("Allow"))
		return
	}
	NotFound(w, r)
}

// NotFound writes the JSON 404 body.
func NotFound(w http.ResponseWriter, r *http.Request) {
	respond.Message(w, r, http.StatusNotFound, "resource not found")
}

// MethodNotAllowed writes the JSON 405 body with the Allow header.
func MethodNotAllowed(w http.ResponseWriter, r *http.Request, allow string) {
	if allow != "" {
		w.Header().Set("Allow", allow)
	}
	respond.Message(w, r, http.StatusMethodNotAllowed, "method not allowed")
}

// headerCapture records the status and headers ServeMux would have written.
type headerCapture struct {
	header http.Header
	status int
}

func (c *headerCapture) Header() http.Header         { return c.header }
func (c *headerCapture) Write(b []byte) (int, error) { return len(b), nil }
func (c *headerCapture) WriteHeader(status int)      { c.status = status }
