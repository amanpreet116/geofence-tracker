package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// square is a valid closed ring around downtown San Francisco.
var square = [][]float64{
	{37.7749, -122.4194},
	{37.7849, -122.4194},
	{37.7849, -122.4094},
	{37.7749, -122.4094},
	{37.7749, -122.4194},
}

func validInput() CreateGeofenceInput {
	return CreateGeofenceInput{Name: "Downtown", Coordinates: square, Category: "delivery_zone"}
}

func TestValidateCreateGeofence(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(in *CreateGeofenceInput)
		wantFields map[string]string // exact expected details; nil means valid
	}{
		{name: "valid", mutate: func(*CreateGeofenceInput) {}},
		{
			name:   "valid with whitespace trimmed",
			mutate: func(in *CreateGeofenceInput) { in.Name = "  Downtown  "; in.Category = " toll_zone " },
		},
		{
			name:       "missing name",
			mutate:     func(in *CreateGeofenceInput) { in.Name = "   " },
			wantFields: map[string]string{"name": "is required"},
		},
		{
			name:       "name too long",
			mutate:     func(in *CreateGeofenceInput) { in.Name = strings.Repeat("é", 201) },
			wantFields: map[string]string{"name": "must be at most 200 characters"},
		},
		{
			name:       "missing category",
			mutate:     func(in *CreateGeofenceInput) { in.Category = "" },
			wantFields: map[string]string{"category": "is required"},
		},
		{
			name:   "unknown category",
			mutate: func(in *CreateGeofenceInput) { in.Category = "parking" },
			wantFields: map[string]string{
				"category": "must be one of delivery_zone, restricted_zone, toll_zone, customer_area",
			},
		},
		{
			name:       "missing coordinates",
			mutate:     func(in *CreateGeofenceInput) { in.Coordinates = nil },
			wantFields: map[string]string{"coordinates": "is required"},
		},
		{
			name: "open polygon",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{37.7749, -122.4194}, {37.7849, -122.4194}, {37.7849, -122.4094}, {37.7749, -122.4094}}
			},
			wantFields: map[string]string{"coordinates": "polygon must be closed: first and last points must be identical"},
		},
		{
			name: "only 3 points",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{37.7749, -122.4194}, {37.7849, -122.4194}, {37.7749, -122.4194}}
			},
			wantFields: map[string]string{"coordinates": "must contain at least 4 points (3 unique points plus the closing point)"},
		},
		{
			name: "4 points but only 2 unique",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{1, 1}, {2, 2}, {2, 2}, {1, 1}}
			},
			wantFields: map[string]string{"coordinates": "must contain at least 3 unique points"},
		},
		{
			name: "out of range latitude and longitude",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{91, 0}, {10, 181}, {-90.5, -180.5}, {91, 0}}
			},
			wantFields: map[string]string{
				"coordinates[0]": "latitude must be between -90 and 90",
				"coordinates[1]": "longitude must be between -180 and 180",
				"coordinates[2]": "latitude must be between -90 and 90; longitude must be between -180 and 180",
				"coordinates[3]": "latitude must be between -90 and 90",
			},
		},
		{
			name: "boundary values are allowed",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{-90, -180}, {90, -180}, {90, 180}, {-90, -180}}
			},
		},
		{
			name: "wrong pair length",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{37.7749, -122.4194}, {37.7849}, {37.7849, -122.4094, 5}, {}, {37.7749, -122.4194}}
			},
			wantFields: map[string]string{
				"coordinates[1]": "must be a [latitude, longitude] pair",
				"coordinates[2]": "must be a [latitude, longitude] pair",
				"coordinates[3]": "must be a [latitude, longitude] pair",
			},
		},
		{
			name: "more than 1000 points",
			mutate: func(in *CreateGeofenceInput) {
				ring := make([][]float64, 0, 1001)
				for i := range 1000 {
					ring = append(ring, []float64{float64(i%80) / 10, float64(i) / 100})
				}
				in.Coordinates = append(ring, ring[0])
			},
			wantFields: map[string]string{"coordinates": "must contain at most 1000 points"},
		},
		{
			name: "exactly 1000 points allowed",
			mutate: func(in *CreateGeofenceInput) {
				ring := make([][]float64, 0, 1000)
				for i := range 999 {
					ring = append(ring, []float64{float64(i) / 1000, float64(i*i%997) / 1000})
				}
				in.Coordinates = append(ring, ring[0])
			},
		},
		{
			name: "crosses the antimeridian",
			mutate: func(in *CreateGeofenceInput) {
				in.Coordinates = [][]float64{{-17, 179}, {-17, -179}, {-16, -179}, {-16, 179}, {-17, 179}}
			},
			wantFields: map[string]string{
				"coordinates": "polygon must not cross the antimeridian (±180° longitude); split it into two geofences",
			},
		},
		{
			name: "all errors reported together",
			mutate: func(in *CreateGeofenceInput) {
				in.Name, in.Category = "", "nope"
				in.Coordinates = [][]float64{{100, 0}, {1, 1}}
			},
			wantFields: map[string]string{
				"name":           "is required",
				"category":       "must be one of delivery_zone, restricted_zone, toll_zone, customer_area",
				"coordinates[0]": "latitude must be between -90 and 90",
				"coordinates":    "must contain at least 4 points (3 unique points plus the closing point)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validInput()
			tt.mutate(&in)
			g, err := ValidateCreateGeofence(in)

			if tt.wantFields == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if g.Name != strings.TrimSpace(in.Name) || string(g.Category) != strings.TrimSpace(in.Category) {
					t.Errorf("not normalized: %+v", g)
				}
				if len(g.Coordinates) != len(in.Coordinates) || g.Coordinates[1].Lat != in.Coordinates[1][0] || g.Coordinates[1].Lng != in.Coordinates[1][1] {
					t.Errorf("coordinates not preserved in [lat, lng] order: %+v", g.Coordinates)
				}
				return
			}
			assertFields(t, err, tt.wantFields)
		})
	}
}

func TestValidateCategoryFilter(t *testing.T) {
	tests := []struct {
		raw     string
		want    model.Category
		wantErr bool
	}{
		{raw: "", want: ""},
		{raw: "restricted_zone", want: model.CategoryRestrictedZone},
		{raw: " customer_area ", want: model.CategoryCustomerArea},
		{raw: "Restricted_Zone", wantErr: true},
		{raw: "bogus", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ValidateCategoryFilter(tt.raw)
			if tt.wantErr {
				if _, ok := errors.AsType[*model.ValidationError](err); !ok {
					t.Fatalf("err = %v, want *ValidationError", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("got (%q, %v), want %q", got, err, tt.want)
			}
		})
	}
}

func assertFields(t *testing.T, err error, want map[string]string) {
	t.Helper()
	ve, ok := errors.AsType[*model.ValidationError](err)
	if !ok {
		t.Fatalf("err = %v, want *ValidationError", err)
	}
	if len(ve.Fields) != len(want) {
		t.Errorf("fields = %v, want %v", ve.Fields, want)
	}
	for k, w := range want {
		if ve.Fields[k] != w {
			t.Errorf("fields[%q] = %q, want %q", k, ve.Fields[k], w)
		}
	}
}
