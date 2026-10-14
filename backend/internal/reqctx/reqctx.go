// Package reqctx stores per-request values (start time, request ID) in the
// context. It sits below both middleware and respond to avoid import cycles.
package reqctx

import (
	"context"
	"log/slog"
	"time"
)

type ctxKey int

const (
	startKey ctxKey = iota
	requestIDKey
)

// WithStart records when request handling began.
func WithStart(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, startKey, t)
}

// Start returns the time recorded by WithStart.
func Start(ctx context.Context) (time.Time, bool) {
	t, ok := ctx.Value(startKey).(time.Time)
	return t, ok
}

// WithRequestID stores the request's correlation ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestID returns the request's correlation ID, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// LogHandler adds request_id to every record logged with a request context,
// so callers never have to pass it explicitly.
type LogHandler struct {
	slog.Handler
}

func (h LogHandler) Handle(ctx context.Context, r slog.Record) error {
	if id := RequestID(ctx); id != "" {
		r.AddAttrs(slog.String("request_id", id))
	}
	return h.Handler.Handle(ctx, r)
}

func (h LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogHandler{h.Handler.WithAttrs(attrs)}
}

func (h LogHandler) WithGroup(name string) slog.Handler {
	return LogHandler{h.Handler.WithGroup(name)}
}
