package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
)

type fakeVehicleService struct {
	stored []model.Vehicle
	fleet  []model.FleetVehicle
}

func (f *fakeVehicleService) Create(_ context.Context, in service.CreateVehicleInput) (model.Vehicle, error) {
	v, err := service.ValidateCreateVehicle(in)
	if err != nil {
		return model.Vehicle{}, err
	}
	v.ID, v.Status, v.CreatedAt = "veh_test", "active", time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC)
	f.stored = append(f.stored, v)
	return v, nil
}

func (f *fakeVehicleService) List(context.Context) ([]model.Vehicle, error) { return f.stored, nil }

func (f *fakeVehicleService) FleetLocations(context.Context) ([]model.FleetVehicle, error) {
	return f.fleet, nil
}

const vehicleJSON = `{"vehicle_number":"ka-01-ab-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}`

func TestCreateAndListVehicles(t *testing.T) {
	router := testRouter(Handlers{Vehicles: NewVehicleHandler(&fakeVehicleService{})})

	rec, _ := serve(t, router, http.MethodGet, "/vehicles", "")
	if !strings.HasPrefix(rec.Body.String(), `{"vehicles":[],`) {
		t.Errorf("empty list body = %s", rec.Body)
	}

	rec, body := serve(t, router, http.MethodPost, "/vehicles", vehicleJSON)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	want := map[string]any{"id": "veh_test", "vehicle_number": "KA-01-AB-1234", "status": "active"}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	if len(body) != len(want)+1 {
		t.Errorf("unexpected fields in %v", body)
	}

	rec, _ = serve(t, router, http.MethodGet, "/vehicles", "")
	var list struct {
		Vehicles []map[string]any `json:"vehicles"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Vehicles) != 1 {
		t.Fatalf("got %d vehicles", len(list.Vehicles))
	}
	for _, k := range []string{"id", "vehicle_number", "driver_name", "vehicle_type", "phone", "status", "created_at"} {
		if _, ok := list.Vehicles[0][k]; !ok {
			t.Errorf("missing field %q", k)
		}
	}
	if list.Vehicles[0]["created_at"] != "2025-01-15T09:00:00Z" {
		t.Errorf("created_at = %v", list.Vehicles[0]["created_at"])
	}
}

func TestCreateVehicleValidation(t *testing.T) {
	rec, body := serve(t, testRouter(Handlers{Vehicles: NewVehicleHandler(&fakeVehicleService{})}), http.MethodPost, "/vehicles", `{"vehicle_number":"x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	details, _ := body["details"].(map[string]any)
	if len(details) != 4 {
		t.Errorf("want all four fields reported, got %v", details)
	}
}

func TestFleetLocations(t *testing.T) {
	svc := &fakeVehicleService{fleet: []model.FleetVehicle{
		{
			Vehicle:            model.Vehicle{ID: "veh_a", VehicleNumber: "A1", DriverName: "Ann", VehicleType: "car", Status: "active"},
			LastLocation:       &model.LocationFix{LatLng: model.LatLng{Lat: 37.78, Lng: -122.41}, RecordedAt: time.Date(2025, 1, 15, 10, 35, 0, 0, time.UTC)},
			CurrentGeofenceIDs: []string{"geo_1"},
		},
		{Vehicle: model.Vehicle{ID: "veh_b", VehicleNumber: "B2", Status: "active"}}, // never reported
	}}
	rec, _ := serve(t, testRouter(Handlers{Vehicles: NewVehicleHandler(svc)}), http.MethodGet, "/fleet/locations", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	got := rec.Body.String()
	for _, want := range []string{
		`"current_location":{"latitude":37.78,"longitude":-122.41,"timestamp":"2025-01-15T10:35:00Z"},"current_geofence_ids":["geo_1"]`,
		`"current_location":null,"current_geofence_ids":[]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("body missing %s\nbody: %s", want, got)
		}
	}
}
