// Command server runs the geofencing API and real-time alert server.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/config"
	"github.com/amanpreet116/geofence-tracker/backend/internal/db"
	"github.com/amanpreet116/geofence-tracker/backend/internal/handler"
	"github.com/amanpreet116/geofence-tracker/backend/internal/middleware"
	"github.com/amanpreet116/geofence-tracker/backend/internal/realtime"
	"github.com/amanpreet116/geofence-tracker/backend/internal/repository"
	"github.com/amanpreet116/geofence-tracker/backend/internal/reqctx"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

const (
	dbConnectTimeout = 60 * time.Second
	shutdownTimeout  = 10 * time.Second
	alertWorkers     = 4
	alertQueueSize   = 1024
)

func main() {
	// `server healthcheck` probes GET /health on the local server and exits
	// 0/1. The runtime image is distroless (no shell, curl or wget), so the
	// container healthcheck reuses this binary.
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := slog.New(reqctx.LogHandler{Handler: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})})
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancel := context.WithTimeout(ctx, dbConnectTimeout)
	pool, err := db.Connect(connectCtx, cfg.DatabaseURL, logger)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	// Real-time pipeline. The hub has its own context: it must outlive the
	// HTTP server so queued alerts can still be delivered while draining.
	hub := realtime.NewHub(cfg.AllowedOrigins, logger)
	hubCtx, stopHub := context.WithCancel(context.Background())
	defer stopHub()
	go hub.Run(hubCtx)

	notifications := repository.NewNotificationRepository(pool)
	alertService := service.NewAlertService(repository.NewAlertRepository(pool), notifications)
	dispatcher := service.NewAlertDispatcher(alertService, notifications, hub,
		service.DispatcherConfig{Workers: alertWorkers, QueueSize: alertQueueSize}, logger)
	dispatcher.Start()

	router := handler.NewRouter(handler.Handlers{
		Health:    handler.NewHealthHandler(pool),
		Geofences: handler.NewGeofenceHandler(service.NewGeofenceService(repository.NewGeofenceRepository(pool))),
		Vehicles:  handler.NewVehicleHandler(service.NewVehicleService(repository.NewVehicleRepository(pool))),
		Locations: handler.NewLocationHandler(service.NewLocationService(repository.NewLocationRepository(pool), dispatcher)),
		Alerts:    handler.NewAlertHandler(alertService),
		History:   handler.NewViolationHandler(service.NewViolationService(repository.NewViolationRepository(pool))),
		WS:        http.HandlerFunc(hub.ServeWS),
	})
	srv := &http.Server{
		Addr: cfg.Addr(),
		Handler: middleware.Chain(router,
			middleware.Timing,
			middleware.RequestID,
			middleware.AccessLog(logger),
			middleware.Recover(logger),
			middleware.CORS(cfg.AllowedOrigins),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	// Shutdown order: stop accepting HTTP → drain queued alerts (needs DB and
	// hub) → close WebSocket clients → (deferred) close the DB pool.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	var shutdownErr error
	if err := srv.Shutdown(shutdownCtx); err != nil {
		shutdownErr = fmt.Errorf("http shutdown: %w", err)
	}
	if err := dispatcher.Shutdown(shutdownCtx); err != nil {
		logger.Warn("alert queue not fully drained", "error", err)
	}
	stopHub()
	select {
	case <-hub.Done():
	case <-shutdownCtx.Done():
	}
	logger.Info("server stopped")
	return shutdownErr
}

func healthcheck() int {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://127.0.0.1:" + port + "/health")
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
