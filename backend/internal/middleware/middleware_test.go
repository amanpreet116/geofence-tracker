package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/handler/respond"
	"github.com/amanpreet116/geofence-tracker/backend/internal/reqctx"
)

var numericString = regexp.MustCompile(`^[0-9]+$`)

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// stack mirrors the production order in cmd/server/main.go.
func stack(logger *slog.Logger, origins []string, h http.Handler) http.Handler {
	return Chain(h, Timing, RequestID, AccessLog(logger), Recover(logger), CORS(origins))
}

type okBody struct {
	Message string `json:"message"`
	respond.Timed
}

func TestSuccessIncludesTimeNS(t *testing.T) {
	h := stack(discardLogger(), []string{"*"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond)
		respond.JSON(w, r, http.StatusOK, &okBody{Message: "hi"})
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ns, ok := body["time_ns"].(string)
	if !ok || !numericString.MatchString(ns) {
		t.Fatalf("time_ns = %#v, want numeric string", body["time_ns"])
	}
	if len(ns) < 7 { // >= 1ms = 1,000,000ns proves the start came from Timing
		t.Errorf("time_ns = %s, want >= 1ms", ns)
	}
}

func TestRecoverReturnsJSON500(t *testing.T) {
	h := stack(discardLogger(), []string{"*"}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	if body["error"] != "internal server error" {
		t.Errorf("error = %v", body["error"])
	}
	if ns, _ := body["time_ns"].(string); !numericString.MatchString(ns) {
		t.Errorf("time_ns = %#v, want numeric string", body["time_ns"])
	}
}

func TestRecoverAfterPartialWriteDoesNotAppend(t *testing.T) {
	h := Recover(discardLogger())(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic("late")
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusAccepted || rec.Body.String() != "partial" {
		t.Errorf("got %d %q, want untouched partial response", rec.Code, rec.Body.String())
	}
}

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
		wantSame bool
	}{
		{name: "generated when absent", incoming: ""},
		{name: "propagated when valid", incoming: "abc-123_X.y", wantSame: true},
		{name: "replaced when invalid", incoming: "bad id\nwith newline"},
		{name: "replaced when too long", incoming: strings.Repeat("a", 129)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen string
			h := RequestID(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				seen = reqctx.RequestID(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.incoming != "" {
				req.Header.Set(RequestIDHeader, tt.incoming)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if got := rec.Header().Get(RequestIDHeader); got != seen || got == "" {
				t.Fatalf("header %q, context %q: want equal and non-empty", got, seen)
			}
			if tt.wantSame && seen != tt.incoming {
				t.Errorf("id = %q, want propagated %q", seen, tt.incoming)
			}
			if !tt.wantSame && !strings.HasPrefix(seen, "req_") {
				t.Errorf("id = %q, want generated req_ id", seen)
			}
		})
	}
}

func TestAccessLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(reqctx.LogHandler{Handler: slog.NewJSONHandler(&buf, nil)})
	h := Chain(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond.Message(w, r, http.StatusTeapot, "nope")
	}), Timing, RequestID, AccessLog(logger))

	req := httptest.NewRequest(http.MethodPost, "/vehicles", nil)
	req.Header.Set(RequestIDHeader, "trace-1")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line %q: %v", buf.String(), err)
	}
	for k, want := range map[string]any{"msg": "request", "method": "POST", "path": "/vehicles", "status": float64(418), "request_id": "trace-1"} {
		if line[k] != want {
			t.Errorf("%s = %v, want %v", k, line[k], want)
		}
	}
	if _, ok := line["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing: %v", line)
	}
}

func TestCORS(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	tests := []struct {
		name        string
		origins     []string
		method      string
		origin      string
		preflight   bool
		wantStatus  int
		wantAllowed string
	}{
		{name: "wildcard simple", origins: []string{"*"}, method: http.MethodGet, origin: "http://x.dev", wantStatus: 200, wantAllowed: "*"},
		{name: "listed origin echoed", origins: []string{"http://a.dev"}, method: http.MethodGet, origin: "http://a.dev", wantStatus: 200, wantAllowed: "http://a.dev"},
		{name: "unlisted origin no header", origins: []string{"http://a.dev"}, method: http.MethodGet, origin: "http://evil.dev", wantStatus: 200},
		{name: "preflight allowed", origins: []string{"http://a.dev"}, method: http.MethodOptions, origin: "http://a.dev", preflight: true, wantStatus: 204, wantAllowed: "http://a.dev"},
		{name: "preflight rejected", origins: []string{"http://a.dev"}, method: http.MethodOptions, origin: "http://evil.dev", preflight: true, wantStatus: 204},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/geofences", nil)
			req.Header.Set("Origin", tt.origin)
			if tt.preflight {
				req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			}
			rec := httptest.NewRecorder()
			CORS(tt.origins)(next).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowed {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantAllowed)
			}
			hasMethods := rec.Header().Get("Access-Control-Allow-Methods") != ""
			if hasMethods != (tt.preflight && tt.wantAllowed != "") {
				t.Errorf("Allow-Methods present = %v", hasMethods)
			}
		})
	}
}
