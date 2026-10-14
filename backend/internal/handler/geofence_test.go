package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

// fakeGeofenceService runs the real validation but keeps data in memory.
type fakeGeofenceService struct {
	stored []model.Geofence
}

func (f *fakeGeofenceService) Create(_ context.Context, in service.CreateGeofenceInput) (model.Geofence, error) {
	g, err := service.ValidateCreateGeofence(in)
	if err != nil {
		return model.Geofence{}, err
	}
	g.ID, g.Status = "geo_test", "active"
	g.CreatedAt = time.Date(2025, 1, 15, 10, 30, 0, 0, time.FixedZone("IST", 19800))
	f.stored = append(f.stored, g)
	return g, nil
}

func (f *fakeGeofenceService) List(_ context.Context, category string) ([]model.Geofence, error) {
	if _, err := service.ValidateCategoryFilter(category); err != nil {
		return nil, err
	}
	return f.stored, nil
}

func serve(t *testing.T, h http.Handler, method, target, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	if ns, _ := decoded["time_ns"].(string); !numericString.MatchString(ns) {
		t.Errorf("time_ns = %#v, want numeric string", decoded["time_ns"])
	}
	return rec, decoded
}

const downtownJSON = `{
	"name": "Downtown Delivery Zone",
	"description": "Main delivery area for downtown customers",
	"coordinates": [[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],
	"category": "delivery_zone"
}`

func TestCreateGeofence(t *testing.T) {
	router := testRouter(Handlers{})

	rec, body := serve(t, router, http.MethodPost, "/geofences", downtownJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	want := map[string]any{"id": "geo_test", "name": "Downtown Delivery Zone", "status": "active"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	if len(body) != len(want)+1 { // + time_ns
		t.Errorf("unexpected fields in %v", body)
	}
}

func TestCreateGeofenceValidationDetails(t *testing.T) {
	router := testRouter(Handlers{})

	rec, body := serve(t, router, http.MethodPost, "/geofences",
		`{"name":"","coordinates":[[95,0],[1,1],[2,2],[3,3]],"category":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	details, _ := body["details"].(map[string]any)
	for _, field := range []string{"name", "category", "coordinates[0]"} {
		if _, ok := details[field]; !ok {
			t.Errorf("details missing %q: %v", field, details)
		}
	}
}

func TestListGeofences(t *testing.T) {
	svc := &fakeGeofenceService{}
	router := testRouter(Handlers{Geofences: NewGeofenceHandler(svc)})

	// Empty list must be [], not null.
	rec, _ := serve(t, router, http.MethodGet, "/geofences", "")
	if !strings.HasPrefix(rec.Body.String(), `{"geofences":[],`) {
		t.Errorf("empty list body = %s", rec.Body)
	}

	serve(t, router, http.MethodPost, "/geofences", downtownJSON)
	rec, _ = serve(t, router, http.MethodGet, "/geofences?category=delivery_zone", "")
	var resp struct {
		Geofences []map[string]any `json:"geofences"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Geofences) != 1 {
		t.Fatalf("got %d geofences", len(resp.Geofences))
	}
	g := resp.Geofences[0]
	if g["created_at"] != "2025-01-15T05:00:00Z" {
		t.Errorf("created_at = %v, want RFC3339 UTC", g["created_at"])
	}
	first, _ := g["coordinates"].([]any)[0].([]any)
	if first[0] != 37.7749 || first[1] != -122.4194 {
		t.Errorf("first coordinate = %v, want [lat, lng]", first)
	}
	for _, k := range []string{"id", "name", "description", "coordinates", "category", "created_at"} {
		if _, ok := g[k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}

	rec, body := serve(t, router, http.MethodGet, "/geofences?category=bogus", "")
	if rec.Code != http.StatusBadRequest || body["details"] == nil {
		t.Errorf("invalid category: status %d body %v", rec.Code, body)
	}
}
