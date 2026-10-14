package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

func validVehicle() CreateVehicleInput {
	return CreateVehicleInput{VehicleNumber: "KA-01-AB-1234", DriverName: "John Doe", VehicleType: "truck", Phone: "+1234567890"}
}

func TestValidateCreateVehicle(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(in *CreateVehicleInput)
		want       *model.Vehicle    // checked when valid
		wantFields map[string]string // nil means valid
	}{
		{
			name:   "spec example",
			mutate: func(*CreateVehicleInput) {},
			want:   &model.Vehicle{VehicleNumber: "KA-01-AB-1234", DriverName: "John Doe", VehicleType: "truck", Phone: "+1234567890"},
		},
		{
			name: "normalized: trimmed, upper-cased, whitespace collapsed",
			mutate: func(in *CreateVehicleInput) {
				*in = CreateVehicleInput{VehicleNumber: "  ka 01   ab 1234 ", DriverName: " Jane ", VehicleType: " Van ", Phone: " +44 (20) 7946-0958 "}
			},
			want: &model.Vehicle{VehicleNumber: "KA 01 AB 1234", DriverName: "Jane", VehicleType: "Van", Phone: "+44 (20) 7946-0958"},
		},
		{name: "3-char number ok", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = "A1B" }},
		{name: "20-char number ok", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = strings.Repeat("A", 20) }},
		{name: "7-digit phone ok", mutate: func(in *CreateVehicleInput) { in.Phone = "1234567" }},
		{name: "15-digit phone ok", mutate: func(in *CreateVehicleInput) { in.Phone = "+123456789012345" }},
		{name: "free-text vehicle type", mutate: func(in *CreateVehicleInput) { in.VehicleType = "electric cargo bike" }},
		{
			name:       "all fields missing",
			mutate:     func(in *CreateVehicleInput) { *in = CreateVehicleInput{VehicleNumber: " ", DriverName: "\t"} },
			wantFields: map[string]string{"vehicle_number": "is required", "driver_name": "is required", "vehicle_type": "is required", "phone": "is required"},
		},
		{name: "number too short", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = "AB" }, wantFields: numberErr},
		{name: "number too long", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = strings.Repeat("A", 21) }, wantFields: numberErr},
		{name: "number bad chars", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = "KA_01/AB" }, wantFields: numberErr},
		{name: "number leading hyphen", mutate: func(in *CreateVehicleInput) { in.VehicleNumber = "-KA01" }, wantFields: numberErr},
		{name: "phone too short", mutate: func(in *CreateVehicleInput) { in.Phone = "123456" }, wantFields: phoneErr},
		{name: "phone too long", mutate: func(in *CreateVehicleInput) { in.Phone = "+1234567890123456" }, wantFields: phoneErr},
		{name: "phone letters", mutate: func(in *CreateVehicleInput) { in.Phone = "call-me-maybe" }, wantFields: phoneErr},
		{name: "phone plus in middle", mutate: func(in *CreateVehicleInput) { in.Phone = "12+34567890" }, wantFields: phoneErr},
		{
			name:       "driver name too long",
			mutate:     func(in *CreateVehicleInput) { in.DriverName = strings.Repeat("x", 101) },
			wantFields: map[string]string{"driver_name": "must be at most 100 characters"},
		},
		{
			name:       "vehicle type too long",
			mutate:     func(in *CreateVehicleInput) { in.VehicleType = strings.Repeat("x", 51) },
			wantFields: map[string]string{"vehicle_type": "must be at most 50 characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validVehicle()
			tt.mutate(&in)
			v, err := ValidateCreateVehicle(in)
			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want != nil && v != *tt.want {
				t.Errorf("got %+v, want %+v", v, *tt.want)
			}
		})
	}
}

var (
	numberErr = map[string]string{"vehicle_number": "must be 3-20 characters of letters, digits, hyphens or spaces, starting and ending with a letter or digit"}
	phoneErr  = map[string]string{"phone": "must be 7-15 digits with an optional leading +"}
)

type fakeVehicleRepo struct{ createErr error }

func (f *fakeVehicleRepo) CreateVehicle(_ context.Context, _ *model.Vehicle) error {
	return f.createErr
}
func (f *fakeVehicleRepo) ListVehicles(context.Context) ([]model.Vehicle, error) { return nil, nil }
func (f *fakeVehicleRepo) ListFleetLocations(context.Context) ([]model.FleetVehicle, error) {
	return nil, nil
}

func TestVehicleServiceCreate(t *testing.T) {
	v, err := NewVehicleService(&fakeVehicleRepo{}).Create(context.Background(), validVehicle())
	if err != nil || !strings.HasPrefix(v.ID, model.PrefixVehicle) || v.Status != "active" {
		t.Errorf("got %+v, %v", v, err)
	}

	conflict := model.NewError(model.ErrConflict, "dup")
	_, err = NewVehicleService(&fakeVehicleRepo{createErr: conflict}).Create(context.Background(), validVehicle())
	if !errors.Is(err, model.ErrConflict) {
		t.Errorf("err = %v, want conflict preserved through wrapping", err)
	}
}
