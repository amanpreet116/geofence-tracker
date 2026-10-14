package service

import (
	"testing"
	"time"
)

func TestValidateViolationQuery(t *testing.T) {
	jan1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	jan31 := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	tests := []struct {
		name       string
		in         ViolationQueryInput
		want       ViolationFilter
		wantFields map[string]string
	}{
		{name: "defaults", want: ViolationFilter{Limit: 50}},
		{
			name: "all filters",
			in: ViolationQueryInput{VehicleID: " veh_1 ", GeofenceID: "geo_1", StartDate: "2025-01-01T00:00:00Z",
				EndDate: "2025-01-31T23:59:59Z", Limit: "100", Offset: "200"},
			want: ViolationFilter{VehicleID: "veh_1", GeofenceID: "geo_1", Start: &jan1, End: &jan31, Limit: 100, Offset: 200},
		},
		{
			name: "offset dates normalized to UTC",
			in:   ViolationQueryInput{StartDate: "2025-01-01T05:30:00+05:30"},
			want: ViolationFilter{Start: &jan1, Limit: 50},
		},
		{name: "limit 500 kept", in: ViolationQueryInput{Limit: "500"}, want: ViolationFilter{Limit: 500}},
		{name: "limit 501 clamped", in: ViolationQueryInput{Limit: "501"}, want: ViolationFilter{Limit: 500}},
		{name: "huge limit clamped", in: ViolationQueryInput{Limit: "1000000"}, want: ViolationFilter{Limit: 500}},
		{name: "limit 1", in: ViolationQueryInput{Limit: "1"}, want: ViolationFilter{Limit: 1}},
		{name: "start equals end allowed", in: ViolationQueryInput{StartDate: "2025-01-01T00:00:00Z", EndDate: "2025-01-01T00:00:00Z"},
			want: ViolationFilter{Start: &jan1, End: &jan1, Limit: 50}},
		{name: "limit zero", in: ViolationQueryInput{Limit: "0"}, wantFields: map[string]string{"limit": "must be a positive integer (max 500)"}},
		{name: "limit negative", in: ViolationQueryInput{Limit: "-5"}, wantFields: map[string]string{"limit": "must be a positive integer (max 500)"}},
		{name: "limit not a number", in: ViolationQueryInput{Limit: "ten"}, wantFields: map[string]string{"limit": "must be a positive integer (max 500)"}},
		{name: "limit decimal", in: ViolationQueryInput{Limit: "10.5"}, wantFields: map[string]string{"limit": "must be a positive integer (max 500)"}},
		{name: "offset negative", in: ViolationQueryInput{Offset: "-1"}, wantFields: map[string]string{"offset": "must be a non-negative integer"}},
		{
			name:       "date without time",
			in:         ViolationQueryInput{StartDate: "2025-01-01"},
			wantFields: map[string]string{"start_date": "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-01T00:00:00Z"},
		},
		{
			name:       "date without zone",
			in:         ViolationQueryInput{EndDate: "2025-01-01T00:00:00"},
			wantFields: map[string]string{"end_date": "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-01T00:00:00Z"},
		},
		{
			name:       "start after end",
			in:         ViolationQueryInput{StartDate: "2025-02-01T00:00:00Z", EndDate: "2025-01-01T00:00:00Z"},
			wantFields: map[string]string{"start_date": "must not be after end_date"},
		},
		{
			name: "all errors together",
			in:   ViolationQueryInput{StartDate: "yesterday", EndDate: "today", Limit: "x", Offset: "y"},
			wantFields: map[string]string{
				"start_date": "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-01T00:00:00Z",
				"end_date":   "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-01T00:00:00Z",
				"limit":      "must be a positive integer (max 500)",
				"offset":     "must be a non-negative integer",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateViolationQuery(tt.in)
			if tt.wantFields != nil {
				assertFields(t, err, tt.wantFields)
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.VehicleID != tt.want.VehicleID || got.GeofenceID != tt.want.GeofenceID ||
				got.Limit != tt.want.Limit || got.Offset != tt.want.Offset ||
				!timePtrEqual(got.Start, tt.want.Start) || !timePtrEqual(got.End, tt.want.End) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func timePtrEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b) && a.Location() == time.UTC
}
