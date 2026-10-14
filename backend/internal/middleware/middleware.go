// Package middleware provides cross-cutting HTTP middleware.
//
// Recommended order (outermost first): Timing, RequestID, AccessLog,
// Recover, CORS. Timing is outermost so that every response — including
// panics and CORS rejections — has a start time for time_ns.
package middleware

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/reqctx"
)

// RequestIDHeader is read from requests and echoed on responses.
const RequestIDHeader = "X-Request-ID"

const maxRequestIDLen = 128

// Chain applies middlewares so the first listed is the outermost.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for _, mw := range slices.Backward(mws) {
		h = mw(h)
	}
	return h
}

// Timing records the request start time used for time_ns.
func Timing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(reqctx.WithStart(r.Context(), time.Now())))
	})
}

// RequestID propagates a well-formed incoming X-Request-ID or generates one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = model.NewID(model.PrefixRequest)
		}
		w.Header().Set(RequestIDHeader, id)
		next.ServeHTTP(w, r.WithContext(reqctx.WithRequestID(r.Context(), id)))
	})
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, c := range id {
		isAlnum := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
		if !isAlnum && c != '-' && c != '_' && c != '.' {
			return false
		}
	}
	return true
}

// AccessLog logs one structured line per request. request_id is added by
// reqctx.LogHandler.
func AccessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := wrap(w)
			next.ServeHTTP(rw, r)
			logger.InfoContext(r.Context(), "request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.status,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
				"bytes", rw.bytes,
				"remote", r.RemoteAddr,
			)
		})
	}
}

// Recover turns a panic into a JSON 500 (with time_ns) if nothing has been
// written yet; otherwise it can only log.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := wrap(w)
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.ErrorContext(r.Context(), "panic recovered", "panic", rec, "stack", string(debug.Stack()))
				if !rw.wroteHeader {
					respond.Message(rw, r, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// CORS allows the configured origins ("*" allows any) and answers preflights.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := slices.Contains(allowedOrigins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			allowed := origin != "" && (allowAll || slices.Contains(allowedOrigins, origin))
			h := w.Header()
			if !allowAll {
				h.Add("Vary", "Origin")
			}
			if allowed {
				if allowAll {
					h.Set("Access-Control-Allow-Origin", "*")
				} else {
					h.Set("Access-Control-Allow-Origin", origin)
				}
				h.Set("Access-Control-Expose-Headers", RequestIDHeader)
			}

			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if allowed {
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, "+RequestIDHeader)
					h.Set("Access-Control-Max-Age", "600")
				}
				// Without allow headers the browser blocks the actual request.
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter records status and bytes written. It forwards Hijack (for
// the WebSocket upgrade) and Unwrap (for http.ResponseController).
type responseWriter struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

func wrap(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status, rw.wroteHeader = code, true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wroteHeader = true
	n, err := rw.ResponseWriter.Write(b)
	rw.bytes += n
	return n, err
}

func (rw *responseWriter) Unwrap() http.ResponseWriter { return rw.ResponseWriter }

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("response writer does not support hijacking")
	}
	rw.status, rw.wroteHeader = http.StatusSwitchingProtocols, true
	return hj.Hijack()
}
