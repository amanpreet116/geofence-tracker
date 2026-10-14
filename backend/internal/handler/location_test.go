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

type fakeLocationService struct {
	result    service.LocationResult
	locations map[string]model.VehicleLocation
}

func (f *fakeLocationService) UpdateLocation(_ context.Context, in service.UpdateLocationInput) (service.LocationResult, error) {
	u, err := service.ValidateLocationUpdate(in, time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC))
	if err != nil {
		return service.LocationResult{}, err
	}
	if _, ok := f.locations[u.VehicleID]; !ok {
		return service.LocationResult{}, model.NewError(model.ErrNotFound, `vehicle "`+u.VehicleID+`" not found`)
	}
	res := f.result
	res.VehicleID = u.VehicleID
	return res, nil
}

func (f *fakeLocationService) GetVehicleLocation(_ context.Context, id string) (model.VehicleLocation, error) {
	vl, ok := f.locations[id]
	if !ok {
		return model.VehicleLocation{}, model.NewError(model.ErrNotFound, `vehicle "`+id+`" not found`)
	}
	return vl, nil
}

func locationRouter() *Router {
	downtown := model.GeofenceRef{ID: "geo_123", Name: "Downtown Delivery Zone", Category: model.CategoryDeliveryZone}
	svc := &fakeLocationService{
		result: service.LocationResult{CurrentGeofences: []model.GeofenceRef{downtown}},
		locations: map[string]model.VehicleLocation{
			"veh_456": {
				Vehicle: model.Vehicle{ID: "veh_456", VehicleNumber: "KA-01-AB-1234"},
				LastLocation: &model.LocationFix{
					LatLng:     model.LatLng{Lat: 37.7849, Lng: -122.4194},
					RecordedAt: time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC),
				},
				CurrentGeofences: []model.GeofenceRef{downtown},
			},
			"veh_new": {Vehicle: model.Vehicle{ID: "veh_new", VehicleNumber: "NEW-1"}},
		},
	}
	return testRouter(Handlers{Locations: NewLocationHandler(svc)})
}

func TestUpdateLocationHandler(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string // prefix, before time_ns
		wantFields []string
	}{
		{
			name:       "spec example",
			body:       `{"vehicle_id":"veh_456","latitude":37.7849,"longitude":-122.4194,"timestamp":"2025-01-15T10:35:00Z"}`,
			wantStatus: http.StatusOK,
			wantBody:   `{"vehicle_id":"veh_456","location_updated":true,"current_geofences":[{"geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","status":"inside"}],"time_ns":"`,
		},
		{
			name:       "unknown vehicle",
			body:       `{"vehicle_id":"veh_nope","latitude":1,"longitude":1,"timestamp":"2025-01-15T10:35:00Z"}`,
			wantStatus: http.StatusNotFound,
			wantBody:   `{"error":"vehicle \"veh_nope\" not found","time_ns":"`,
		},
		{
			name:       "missing coordinates and bad timestamp",
			body:       `{"vehicle_id":"veh_456","timestamp":"yesterday"}`,
			wantStatus: http.StatusBadRequest,
			wantFields: []string{"latitude", "longitude", "timestamp"},
		},
		{
			name:       "latitude as string",
			body:       `{"vehicle_id":"veh_456","latitude":"37.7","longitude":1,"timestamp":"2025-01-15T10:35:00Z"}`,
			wantStatus: http.StatusBadRequest,
			wantFields: []string{"latitude"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, body := serve(t, locationRouter(), http.MethodPost, "/vehicles/location", tt.body)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if tt.wantBody != "" && !strings.HasPrefix(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %s\nwant prefix %s", rec.Body, tt.wantBody)
			}
			details, _ := body["details"].(map[string]any)
			for _, f := range tt.wantFields {
				if _, ok := details[f]; !ok {
					t.Errorf("details missing %q: %v", f, details)
				}
			}
		})
	}
}

func TestGetVehicleLocationHandler(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name: "with location", path: "/vehicles/location/veh_456", wantStatus: http.StatusOK,
			wantBody: `{"vehicle_id":"veh_456","vehicle_number":"KA-01-AB-1234","current_location":{"latitude":37.7849,"longitude":-122.4194,"timestamp":"2025-01-15T10:35:00Z"},"current_geofences":[{"geofence_id":"geo_123","geofence_name":"Downtown Delivery Zone","category":"delivery_zone"}],"time_ns":"`,
		},
		{
			name: "never reported", path: "/vehicles/location/veh_new", wantStatus: http.StatusOK,
			wantBody: `{"vehicle_id":"veh_new","vehicle_number":"NEW-1","current_location":null,"current_geofences":[],"time_ns":"`,
		},
		{
			name: "unknown vehicle", path: "/vehicles/location/veh_nope", wantStatus: http.StatusNotFound,
			wantBody: `{"error":"vehicle \"veh_nope\" not found","time_ns":"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, _ := serve(t, locationRouter(), http.MethodGet, tt.path, "")
			if rec.Code != tt.wantStatus || !strings.HasPrefix(rec.Body.String(), tt.wantBody) {
				t.Errorf("got %d %s\nwant %d %s…", rec.Code, rec.Body, tt.wantStatus, tt.wantBody)
			}
		})
	}
}
