package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
	"github.com/amanpreet116/geofence-tracker/backend/internal/repository"
	"github.com/amanpreet116/geofence-tracker/backend/internal/service"
	"github.com/amanpreet116/geofence-tracker/backend/internal/testutil"
)

// TestCreateVehicleDuplicateIntegration drives handler → service →
// repository → PostgreSQL and checks a duplicate vehicle_number (differing
// only in case and spacing) becomes 409 via the 23505 unique violation.
func TestCreateVehicleDuplicateIntegration(t *testing.T) {
	pool := testutil.Pool(t)
	router := testRouter(Handlers{Vehicles: NewVehicleHandler(service.NewVehicleService(repository.NewVehicleRepository(pool)))})

	number := "IT-" + strings.ToUpper(model.NewID("")[:8])
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM vehicles WHERE vehicle_number = $1", number)
	})

	first := fmt.Sprintf(`{"vehicle_number":%q,"driver_name":"A","vehicle_type":"van","phone":"+1234567890"}`, number)
	rec, body := serve(t, router, http.MethodPost, "/vehicles", first)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create: status %d, body %v", rec.Code, body)
	}

	dup := fmt.Sprintf(`{"vehicle_number":%q,"driver_name":"B","vehicle_type":"car","phone":"+1987654321"}`,
		"  "+strings.ToLower(number)+" ")
	rec, body = serve(t, router, http.MethodPost, "/vehicles", dup)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate: status %d, want 409; body %v", rec.Code, body)
	}
	if want := fmt.Sprintf("vehicle_number %q is already registered", number); body["error"] != want {
		t.Errorf("error = %q, want %q", body["error"], want)
	}
}
