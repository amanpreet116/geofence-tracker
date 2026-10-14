package respond

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/reqctx"
)

var timeNSLast = regexp.MustCompile(`,"time_ns":"[0-9]+"}\n$`)

type sample struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Timed
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestJSONStampsTimeNS(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	start := time.Now().Add(-5 * time.Millisecond)
	req = req.WithContext(reqctx.WithStart(req.Context(), start))
	rec := httptest.NewRecorder()

	JSON(rec, req, http.StatusCreated, &sample{ID: "geo_1", Name: "zone"})

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	raw := rec.Body.String()
	if !strings.HasPrefix(raw, `{"id":"geo_1","name":"zone",`) || !timeNSLast.MatchString(raw) {
		t.Errorf("body %q: want declared field order with numeric string time_ns last", raw)
	}
	var body struct {
		TimeNS json.Number `json:"time_ns"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	ns, _ := body.TimeNS.Int64()
	if ns < (5 * time.Millisecond).Nanoseconds() {
		t.Errorf("time_ns = %d, want >= 5ms measured from context start", ns)
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
		wantDetails map[string]any
	}{
		{
			name:        "validation with fields",
			err:         fmt.Errorf("create: %w", model.NewValidationError("invalid geofence", map[string]string{"name": "is required"})),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "invalid geofence",
			wantDetails: map[string]any{"name": "is required"},
		},
		{
			name:        "validation without fields omits details",
			err:         model.NewValidationError("", nil),
			wantStatus:  http.StatusBadRequest,
			wantMessage: "validation failed",
		},
		{
			name:        "not found",
			err:         fmt.Errorf("vehicle %q: %w", "veh_x", model.ErrNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: `vehicle "veh_x": not found`,
		},
		{
			name:        "domain error message survives wrapping",
			err:         fmt.Errorf("create vehicle: %w", model.NewError(model.ErrConflict, `vehicle_number "KA-01" is already registered`)),
			wantStatus:  http.StatusConflict,
			wantMessage: `vehicle_number "KA-01" is already registered`,
		},
		{name: "conflict", err: model.ErrConflict, wantStatus: http.StatusConflict, wantMessage: "conflict"},
		{name: "rate limited", err: model.ErrRateLimited, wantStatus: http.StatusTooManyRequests, wantMessage: "rate limit exceeded"},
		{name: "unauthorized", err: model.ErrUnauthorized, wantStatus: http.StatusUnauthorized, wantMessage: "unauthorized"},
		{name: "media type", err: model.ErrUnsupportedMediaType, wantStatus: http.StatusUnsupportedMediaType, wantMessage: "unsupported media type"},
		{name: "too large", err: model.ErrPayloadTooLarge, wantStatus: http.StatusRequestEntityTooLarge, wantMessage: "payload too large"},
		{
			name:        "database unreachable is 503, details hidden",
			err:         fmt.Errorf("list geofences: %w", &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}),
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: unavailableMessage,
		},
		{
			name:        "postgres shutting down is 503",
			err:         fmt.Errorf("query: %w", sqlStateErr("57P01")),
			wantStatus:  http.StatusServiceUnavailable,
			wantMessage: unavailableMessage,
		},
		{
			name:        "other postgres errors stay 500",
			err:         fmt.Errorf("query: %w", sqlStateErr("42P01")),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "internal server error",
		},
		{
			name:        "unknown error is hidden",
			err:         errors.New("pq: connection refused to 10.0.0.5"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			Error(rec, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if !timeNSLast.MatchString(rec.Body.String()) {
				t.Errorf("body %q: want numeric string time_ns", rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body["error"] != tt.wantMessage {
				t.Errorf("error = %q, want %q", body["error"], tt.wantMessage)
			}
			details, present := body["details"]
			if tt.wantDetails == nil && present {
				t.Errorf("details = %v, want omitted", details)
			}
			if tt.wantDetails != nil && fmt.Sprint(details) != fmt.Sprint(tt.wantDetails) {
				t.Errorf("details = %v, want %v", details, tt.wantDetails)
			}
		})
	}
}

type sqlStateErr string

func (e sqlStateErr) Error() string    { return "pg error " + string(e) }
func (e sqlStateErr) SQLState() string { return string(e) }

func TestUnavailableSetsRetryAfter(t *testing.T) {
	rec := httptest.NewRecorder()
	Error(rec, httptest.NewRequest(http.MethodGet, "/", nil), &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED})
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") != "5" {
		t.Errorf("status %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

func TestDecode(t *testing.T) {
	type payload struct {
		Name  string  `json:"name"`
		Count int     `json:"count"`
		Lat   float64 `json:"lat"`
	}

	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     error // sentinel; nil means success
		wantValid   string
		wantFields  map[string]string
	}{
		{name: "ok", contentType: "application/json", body: `{"name":"a","count":2}`},
		{name: "charset param ok", contentType: "application/json; charset=utf-8", body: `{"name":"a"}`},
		{name: "unknown fields ignored", contentType: "application/json", body: `{"name":"a","extra":true}`},
		{name: "missing content type", contentType: "", body: `{}`, wantErr: model.ErrUnsupportedMediaType},
		{name: "form content type", contentType: "application/x-www-form-urlencoded", body: `{}`, wantErr: model.ErrUnsupportedMediaType},
		{name: "empty body", contentType: "application/json", body: ``, wantValid: "request body is required"},
		{name: "syntax error", contentType: "application/json", body: `{"name": }`, wantValid: "malformed JSON at position 10"},
		{name: "truncated", contentType: "application/json", body: `{"name":"a"`, wantValid: "malformed JSON: unexpected end of input"},
		{
			name: "wrong field type", contentType: "application/json", body: `{"count":"two"}`,
			wantValid: `invalid type for field "count"`, wantFields: map[string]string{"count": "must be of type number"},
		},
		{name: "not an object", contentType: "application/json", body: `[1,2]`, wantValid: "request body must be a JSON object"},
		{name: "trailing data", contentType: "application/json", body: `{"name":"a"} {"name":"b"}`, wantValid: "request body must contain a single JSON object"},
		{
			name: "too large", contentType: "application/json",
			body:    `{"name":"` + strings.Repeat("x", MaxBodyBytes) + `"}`,
			wantErr: model.ErrPayloadTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			var dst payload
			err := Decode(httptest.NewRecorder(), req, &dst)

			switch {
			case tt.wantErr != nil:
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("err = %v, want %v", err, tt.wantErr)
				}
			case tt.wantValid != "":
				ve, ok := errors.AsType[*model.ValidationError](err)
				if !ok {
					t.Fatalf("err = %v, want *ValidationError", err)
				}
				if ve.Message != tt.wantValid {
					t.Errorf("message = %q, want %q", ve.Message, tt.wantValid)
				}
				if fmt.Sprint(ve.Fields) != fmt.Sprint(tt.wantFields) {
					t.Errorf("fields = %v, want %v", ve.Fields, tt.wantFields)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if dst.Name != "a" {
					t.Errorf("decoded %+v", dst)
				}
			}
		})
	}
}
