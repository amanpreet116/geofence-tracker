package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// AlertMatcher finds the rules triggered by a batch of events
// (AlertService.Match).
type AlertMatcher interface {
	Match(ctx context.Context, events []model.GeofenceEvent) ([]model.AlertMatch, error)
}

// NotificationStore persists dispatched alerts.
type NotificationStore interface {
	// InsertNotification stores n unless a notification for the same
	// (event, rule) exists; inserted reports whether a row was written.
	InsertNotification(ctx context.Context, n model.AlertNotification) (inserted bool, err error)
}

// Broadcaster delivers a serialized alert to connected clients.
type Broadcaster interface {
	Broadcast(msg []byte)
}

// DispatcherConfig sizes the worker pool.
type DispatcherConfig struct {
	Workers      int           // concurrent workers (default 4)
	QueueSize    int           // buffered batches (default 1024)
	BatchTimeout time.Duration // per-batch DB budget (default 5s)
}

// AlertDispatcher is the real EventDispatcher: committed events are queued
// and a fixed pool of workers matches rules, stores one notification per
// (event, rule), and broadcasts it. The HTTP request only pays for a
// non-blocking channel send.
type AlertDispatcher struct {
	matcher     AlertMatcher
	store       NotificationStore
	broadcaster Broadcaster
	logger      *slog.Logger
	cfg         DispatcherConfig

	queue  chan []model.GeofenceEvent
	wg     sync.WaitGroup
	ctx    context.Context // cancelled only on a hard stop
	cancel context.CancelFunc

	mu     sync.RWMutex // guards closed vs. sends on queue
	closed bool
}

var _ EventDispatcher = (*AlertDispatcher)(nil)

func NewAlertDispatcher(matcher AlertMatcher, store NotificationStore, b Broadcaster, cfg DispatcherConfig, logger *slog.Logger) *AlertDispatcher {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 1024
	}
	if cfg.BatchTimeout <= 0 {
		cfg.BatchTimeout = 5 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &AlertDispatcher{
		matcher: matcher, store: store, broadcaster: b, logger: logger, cfg: cfg,
		queue: make(chan []model.GeofenceEvent, cfg.QueueSize),
		ctx:   ctx, cancel: cancel,
	}
}

// Start launches the worker pool. Call once.
func (d *AlertDispatcher) Start() {
	for range d.cfg.Workers {
		d.wg.Go(func() {
			for events := range d.queue {
				d.process(events)
			}
		})
	}
}

// Dispatch enqueues events without blocking. If the queue is full (sustained
// overload) it logs a warning and processes the batch on the caller's
// goroutine: backpressure on that one request is preferable to silently
// losing alerts for events that are already committed.
func (d *AlertDispatcher) Dispatch(events []model.GeofenceEvent) {
	if len(events) == 0 {
		return
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.closed {
		d.logger.Warn("alert dispatcher stopped; events not dispatched", "events", len(events))
		return
	}
	select {
	case d.queue <- events:
	default:
		d.logger.Warn("alert queue full; dispatching synchronously", "queue_size", d.cfg.QueueSize, "events", len(events))
		d.process(events)
	}
}

// Shutdown stops accepting events and waits for queued batches to finish.
// If ctx expires first, in-flight work is cancelled and ctx's error returned.
func (d *AlertDispatcher) Shutdown(ctx context.Context) error {
	d.mu.Lock()
	if !d.closed {
		d.closed = true
		close(d.queue) // workers drain what's left, then exit
	}
	d.mu.Unlock()

	done := make(chan struct{})
	go func() { d.wg.Wait(); close(done) }()
	select {
	case <-done:
		d.cancel()
		return nil
	case <-ctx.Done():
		d.cancel()
		<-done
		return fmt.Errorf("alert dispatcher drain: %w", ctx.Err())
	}
}

func (d *AlertDispatcher) process(events []model.GeofenceEvent) {
	ctx, cancel := context.WithTimeout(d.ctx, d.cfg.BatchTimeout)
	defer cancel()

	matches, err := d.matcher.Match(ctx, events)
	if err != nil {
		d.logger.Error("match alert rules", "error", err, "events", len(events))
		return
	}
	for _, m := range matches {
		for _, rule := range m.Rules {
			if err := d.notify(ctx, m.Event, rule); err != nil {
				d.logger.Error("dispatch alert", "error", err, "geofence_event_id", m.Event.ID, "alert_rule_id", rule.ID)
			}
		}
	}
}

func (d *AlertDispatcher) notify(ctx context.Context, event model.GeofenceEvent, rule model.AlertRule) error {
	n := model.AlertNotification{
		ID:          model.NewID(model.PrefixEvent),
		AlertRuleID: rule.ID,
		EventID:     event.ID,
	}
	n.Payload = BuildAlertPayload(n.ID, event)

	inserted, err := d.store.InsertNotification(ctx, n)
	if err != nil {
		return fmt.Errorf("store notification: %w", err)
	}
	if !inserted {
		return nil // already dispatched for this (event, rule)
	}
	msg, err := json.Marshal(n.Payload)
	if err != nil {
		return fmt.Errorf("encode alert: %w", err)
	}
	d.broadcaster.Broadcast(msg)
	d.logger.Info("alert dispatched", "event_id", n.ID, "alert_rule_id", rule.ID,
		"vehicle_id", event.VehicleID, "geofence_id", event.Geofence.ID, "event_type", event.EventType)
	return nil
}

// BuildAlertPayload renders event in the exact /ws/alerts message format.
func BuildAlertPayload(notificationID string, e model.GeofenceEvent) model.AlertPayload {
	return model.AlertPayload{
		EventID:   notificationID,
		EventType: string(e.EventType),
		Timestamp: e.OccurredAt.UTC().Format(time.RFC3339),
		Vehicle: model.AlertPayloadVehicle{
			VehicleID:     e.VehicleID,
			VehicleNumber: e.VehicleNumber,
			DriverName:    e.DriverName,
		},
		Geofence: model.AlertPayloadGeofence{
			GeofenceID:   e.Geofence.ID,
			GeofenceName: e.Geofence.Name,
			Category:     string(e.Geofence.Category),
		},
		Location: model.AlertPayloadLocation{Latitude: e.Location.Lat, Longitude: e.Location.Lng},
	}
}
