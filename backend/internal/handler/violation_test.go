package handler

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

type fakeViolationService struct {
	events []model.GeofenceEvent
	total  int
	got    service.ViolationFilter
}

func (f *fakeViolationService) History(_ context.Context, in service.ViolationQueryInput) ([]model.GeofenceEvent, int, error) {
	filter, err := service.ValidateViolationQuery(in)
	if err != nil {
		return nil, 0, err
	}
	f.got = filter
	return f.events, f.total, nil
}

func TestViolationHistoryHandler(t *testing.T) {
	svc := &fakeViolationService{
		total: 245,
		events: []model.GeofenceEvent{{
			ID: "viol_111", EventType: model.EventEntry,
			VehicleID: "veh_456", VehicleNumber: "KA-01-AB-1234", DriverName: "John Doe",
			Geofence:   model.GeofenceRef{ID: "geo_123", Name: "Downtown Delivery Zone", Category: model.CategoryDeliveryZone},
			Location:   model.LatLng{Lat: 37.7849, Lng: -122.4194},
			OccurredAt: time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
		}},
	}
	router := testRouter(Handlers{History: NewViolationHandler(svc)})

	rec, _ := serve(t, router, http.MethodGet, "/violations/history?vehicle_id=veh_456&limit=100", "")
	want := `{"violations":[{"id":"viol_111","vehicle_id":"veh_456","vehicle_number":"KA-01-AB-1234","geofence_id":"geo_123",` +
		`"geofence_name":"Downtown Delivery Zone","event_type":"entry","latitude":37.7849,"longitude":-122.4194,` +
		`"timestamp":"2025-01-15T10:35:00Z"}],"total_count":245,"time_ns":"`
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Body.String(), want) {
		t.Errorf("got %d %s\nwant prefix %s", rec.Code, rec.Body, want)
	}
	if svc.got.VehicleID != "veh_456" || svc.got.Limit != 100 {
		t.Errorf("filter passed = %+v", svc.got)
	}

	serve(t, router, http.MethodGet, "/violations/history?limit=9999&offset=10", "")
	if svc.got.Limit != 500 || svc.got.Offset != 10 {
		t.Errorf("clamp/offset: %+v", svc.got)
	}

	rec, body := serve(t, router, http.MethodGet, "/violations/history?start_date=2025-02-01T00:00:00Z&end_date=2025-01-01T00:00:00Z&limit=abc", "")
	details, _ := body["details"].(map[string]any)
	if rec.Code != http.StatusBadRequest || details["start_date"] == nil || details["limit"] == nil {
		t.Errorf("invalid params: %d %v", rec.Code, body)
	}

	empty := testRouter(Handlers{History: NewViolationHandler(&fakeViolationService{})})
	rec, _ = serve(t, empty, http.MethodGet, "/violations/history", "")
	if !strings.HasPrefix(rec.Body.String(), `{"violations":[],"total_count":0,"time_ns":"`) {
		t.Errorf("empty body = %s", rec.Body)
	}
}
