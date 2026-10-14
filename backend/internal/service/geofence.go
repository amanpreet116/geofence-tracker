package service

import (
	"context"
	"fmt"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// GeofenceRepository is the persistence the geofence service needs.
type GeofenceRepository interface {
	// CheckPolygon asks PostGIS whether ring forms a valid polygon. reason
	// explains why not (coordinates in it are in [lat, lng] order).
	CheckPolygon(ctx context.Context, ring []model.LatLng) (valid bool, reason string, err error)
	CreateGeofence(ctx context.Context, g *model.Geofence) error
	ListGeofences(ctx context.Context, category model.Category) ([]model.Geofence, error)
}

type GeofenceService struct {
	repo GeofenceRepository
}

func NewGeofenceService(repo GeofenceRepository) *GeofenceService {
	return &GeofenceService{repo: repo}
}

// Create validates in (locally, then geometrically in PostGIS) and stores it.
func (s *GeofenceService) Create(ctx context.Context, in CreateGeofenceInput) (model.Geofence, error) {
	g, err := ValidateCreateGeofence(in)
	if err != nil {
		return model.Geofence{}, err
	}

	valid, reason, err := s.repo.CheckPolygon(ctx, g.Coordinates)
	if err != nil {
		return model.Geofence{}, fmt.Errorf("check polygon validity: %w", err)
	}
	if !valid {
		return model.Geofence{}, model.NewValidationError("invalid geofence", map[string]string{
			"coordinates": "polygon is not valid: " + reason,
		})
	}

	g.ID = model.NewID(model.PrefixGeofence)
	g.Status = model.GeofenceStatusActive
	if err := s.repo.CreateGeofence(ctx, &g); err != nil {
		return model.Geofence{}, fmt.Errorf("create geofence: %w", err)
	}
	return g, nil
}

// List returns geofences, newest first, optionally filtered by category.
func (s *GeofenceService) List(ctx context.Context, rawCategory string) ([]model.Geofence, error) {
	category, err := ValidateCategoryFilter(rawCategory)
	if err != nil {
		return nil, err
	}
	geofences, err := s.repo.ListGeofences(ctx, category)
	if err != nil {
		return nil, fmt.Errorf("list geofences: %w", err)
	}
	return geofences, nil
}
