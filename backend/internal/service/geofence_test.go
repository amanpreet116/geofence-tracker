package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

type fakeGeofenceRepo struct {
	valid     bool
	reason    string
	checkErr  error
	created   *model.Geofence
	listCat   model.Category
	listCalls int
}

func (f *fakeGeofenceRepo) CheckPolygon(context.Context, []model.LatLng) (bool, string, error) {
	return f.valid, f.reason, f.checkErr
}

func (f *fakeGeofenceRepo) CreateGeofence(_ context.Context, g *model.Geofence) error {
	g.CreatedAt = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)
	f.created = g
	return nil
}

func (f *fakeGeofenceRepo) ListGeofences(_ context.Context, c model.Category) ([]model.Geofence, error) {
	f.listCat, f.listCalls = c, f.listCalls+1
	return nil, nil
}

func TestGeofenceServiceCreate(t *testing.T) {
	// A bowtie: edges (0,0)-(1,1) and (1,0)-(0,1) cross. Locally it passes
	// every check; only PostGIS can reject it.
	bowtie := [][]float64{{0, 0}, {1, 1}, {1, 0}, {0, 1}, {0, 0}}

	tests := []struct {
		name       string
		in         CreateGeofenceInput
		repo       *fakeGeofenceRepo
		wantFields map[string]string
		wantErr    bool
	}{
		{
			name: "valid polygon is stored",
			in:   validInput(),
			repo: &fakeGeofenceRepo{valid: true},
		},
		{
			name: "bowtie self-intersection rejected with reason",
			in:   CreateGeofenceInput{Name: "Bowtie", Coordinates: bowtie, Category: "toll_zone"},
			repo: &fakeGeofenceRepo{valid: false, reason: "Self-intersection at [0.5, 0.5]"},
			wantFields: map[string]string{
				"coordinates": "polygon is not valid: Self-intersection at [0.5, 0.5]",
			},
		},
		{
			name:       "local validation runs before the database",
			in:         CreateGeofenceInput{Coordinates: bowtie, Category: "toll_zone"},
			repo:       &fakeGeofenceRepo{checkErr: errors.New("must not be called")},
			wantFields: map[string]string{"name": "is required"},
		},
		{
			name:    "database failure is an internal error",
			in:      validInput(),
			repo:    &fakeGeofenceRepo{checkErr: errors.New("connection reset")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGeofenceService(tt.repo).Create(context.Background(), tt.in)
			switch {
			case tt.wantFields != nil:
				assertFields(t, err, tt.wantFields)
				if tt.repo.created != nil {
					t.Error("invalid geofence was stored")
				}
			case tt.wantErr:
				if _, isValidation := errors.AsType[*model.ValidationError](err); err == nil || isValidation {
					t.Errorf("err = %v, want non-validation error", err)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !strings.HasPrefix(g.ID, model.PrefixGeofence) || g.Status != "active" || g.CreatedAt.IsZero() {
					t.Errorf("unexpected geofence: %+v", g)
				}
			}
		})
	}
}

func TestGeofenceServiceList(t *testing.T) {
	repo := &fakeGeofenceRepo{}
	svc := NewGeofenceService(repo)

	if _, err := svc.List(context.Background(), "toll_zone"); err != nil || repo.listCat != model.CategoryTollZone {
		t.Errorf("filter not passed through: err=%v cat=%q", err, repo.listCat)
	}
	if _, err := svc.List(context.Background(), "bogus"); err == nil {
		t.Error("invalid category accepted")
	}
	if repo.listCalls != 1 {
		t.Errorf("repository called %d times, want 1 (invalid filter must not reach it)", repo.listCalls)
	}
}
