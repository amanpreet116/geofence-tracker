package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
)

const healthDBTimeout = 2 * time.Second

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthHandler serves GET /health.
type HealthHandler struct {
	db Pinger
}

func NewHealthHandler(db Pinger) *HealthHandler {
	return &HealthHandler{db: db}
}

type healthResponse struct {
	Status string `json:"status"`
	DB     string `json:"db"`
	respond.Timed
}

func (h *HealthHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthDBTimeout)
	defer cancel()
	if err := h.db.Ping(ctx); err != nil {
		slog.WarnContext(r.Context(), "health check: database unreachable", "error", err)
		respond.JSON(w, r, http.StatusServiceUnavailable, &healthResponse{Status: "degraded", DB: "unavailable"})
		return
	}
	respond.JSON(w, r, http.StatusOK, &healthResponse{Status: "ok", DB: "ok"})
}
