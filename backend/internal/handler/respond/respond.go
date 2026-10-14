// Package respond writes HTTP responses that satisfy the API contract:
// every JSON body carries "time_ns", and errors share one shape and one
// error→status mapping.
//
// time_ns injection: every response struct embeds Timed (as its LAST field),
// and JSON stamps it just before marshalling. We chose this over marshalling
// into a map because encoding/json sorts map keys alphabetically, which would
// scatter time_ns into the middle of the body and reorder every field away
// from the documented examples. Embedding keeps declared field order with
// time_ns last, costs one marshal, and the Payload interface makes "forgot
// time_ns" a compile error rather than a runtime omission.
package respond

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/reqctx"
)

// Timed must be embedded (last) in every response struct.
type Timed struct {
	TimeNS string `json:"time_ns"`
}

func (t *Timed) stampTimeNS(ns string) { t.TimeNS = ns }

// Payload is any pointer to a struct embedding Timed.
type Payload interface {
	stampTimeNS(string)
}

// ErrorBody is the uniform error response.
type ErrorBody struct {
	Error   string `json:"error"`
	Details any    `json:"details,omitempty"`
	Timed
}

const (
	internalErrorMessage = "internal server error"
	unavailableMessage   = "service temporarily unavailable: the database cannot be reached, please retry shortly"
)

// JSON stamps time_ns (elapsed since the Timing middleware recorded the
// request start) on payload and writes it with status.
func JSON(w http.ResponseWriter, r *http.Request, status int, payload Payload) {
	payload.stampTimeNS(elapsedNS(r))
	body, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(r.Context(), "encode response", "error", err)
		status = http.StatusInternalServerError
		fallback := &ErrorBody{Error: internalErrorMessage}
		fallback.stampTimeNS(elapsedNS(r))
		body, _ = json.Marshal(fallback)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

// Message writes an error body with an explicit status (used for 404/405/panics).
func Message(w http.ResponseWriter, r *http.Request, status int, message string) {
	JSON(w, r, status, &ErrorBody{Error: message})
}

// Error maps err to a status code and writes the error body. Unknown errors
// become a generic 500; the real error is logged, never sent.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	status, body := classify(err)
	switch status {
	case http.StatusInternalServerError:
		slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	case http.StatusServiceUnavailable:
		slog.WarnContext(r.Context(), "dependency unavailable", "method", r.Method, "path", r.URL.Path, "error", err)
		w.Header().Set("Retry-After", "5")
	}
	JSON(w, r, status, body)
}

// classify is the single place mapping domain errors to HTTP status codes.
func classify(err error) (int, *ErrorBody) {
	if ve, ok := errors.AsType[*model.ValidationError](err); ok {
		body := &ErrorBody{Error: ve.Error()}
		if len(ve.Fields) > 0 {
			body.Details = ve.Fields
		}
		return http.StatusBadRequest, body
	}

	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, model.ErrRateLimited):
		status = http.StatusTooManyRequests
	case errors.Is(err, model.ErrUnauthorized):
		status = http.StatusUnauthorized
	case errors.Is(err, model.ErrUnsupportedMediaType):
		status = http.StatusUnsupportedMediaType
	case errors.Is(err, model.ErrPayloadTooLarge):
		status = http.StatusRequestEntityTooLarge
	case model.IsUnavailable(err):
		return http.StatusServiceUnavailable, &ErrorBody{Error: unavailableMessage}
	default:
		return status, &ErrorBody{Error: internalErrorMessage}
	}
	// Prefer the domain error's own message over the full wrap chain.
	if de, ok := errors.AsType[*model.Error](err); ok {
		return status, &ErrorBody{Error: de.Message, Details: de.Details}
	}
	return status, &ErrorBody{Error: err.Error()}
}

func elapsedNS(r *http.Request) string {
	start, ok := reqctx.Start(r.Context())
	if !ok {
		// Timing middleware not installed (e.g. a handler unit test).
		start = time.Now()
	}
	return strconv.FormatInt(time.Since(start).Nanoseconds(), 10)
}
