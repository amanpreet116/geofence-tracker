package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

const (
	defaultViolationLimit = 50
	maxViolationLimit     = 500
)

// ViolationQueryInput holds the raw GET /violations/history query params.
type ViolationQueryInput struct {
	VehicleID  string
	GeofenceID string
	StartDate  string
	EndDate    string
	Limit      string
	Offset     string
}

// ViolationFilter is a validated history query. Start/End are inclusive;
// nil means unbounded.
type ViolationFilter struct {
	VehicleID  string
	GeofenceID string
	Start      *time.Time
	End        *time.Time
	Limit      int
	Offset     int
}

// ValidateViolationQuery parses and checks the query. limit defaults to 50;
// values above 500 are clamped to 500, anything non-numeric or < 1 is
// rejected. offset defaults to 0. All problems are reported together.
func ValidateViolationQuery(in ViolationQueryInput) (ViolationFilter, error) {
	errs := fieldErrors{}
	f := ViolationFilter{
		VehicleID:  strings.TrimSpace(in.VehicleID),
		GeofenceID: strings.TrimSpace(in.GeofenceID),
		Limit:      defaultViolationLimit,
	}

	f.Start = parseDateParam(errs, "start_date", in.StartDate)
	f.End = parseDateParam(errs, "end_date", in.EndDate)
	if f.Start != nil && f.End != nil && f.Start.After(*f.End) {
		errs.add("start_date", "must not be after end_date")
	}

	if raw := strings.TrimSpace(in.Limit); raw != "" {
		n, err := strconv.Atoi(raw)
		switch {
		case err != nil || n < 1:
			errs.add("limit", fmt.Sprintf("must be a positive integer (max %d)", maxViolationLimit))
		default:
			f.Limit = min(n, maxViolationLimit)
		}
	}
	if raw := strings.TrimSpace(in.Offset); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			errs.add("offset", "must be a non-negative integer")
		} else {
			f.Offset = n
		}
	}

	if err := errs.err("invalid query parameter"); err != nil {
		return ViolationFilter{}, err
	}
	return f, nil
}

func parseDateParam(errs fieldErrors, field, raw string) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		errs.add(field, "must be an RFC 3339 / ISO 8601 timestamp, e.g. 2025-01-01T00:00:00Z")
		return nil
	}
	t = t.UTC()
	return &t
}

// ViolationRepository is the persistence the violation service needs.
type ViolationRepository interface {
	// ListViolations returns one page of events and the total count of
	// events matching the filter (ignoring limit/offset).
	ListViolations(ctx context.Context, f ViolationFilter) (page []model.GeofenceEvent, total int, err error)
}

type ViolationService struct {
	repo ViolationRepository
}

func NewViolationService(repo ViolationRepository) *ViolationService {
	return &ViolationService{repo: repo}
}

// History returns geofence entry/exit events, newest first.
func (s *ViolationService) History(ctx context.Context, in ViolationQueryInput) ([]model.GeofenceEvent, int, error) {
	f, err := ValidateViolationQuery(in)
	if err != nil {
		return nil, 0, err
	}
	page, total, err := s.repo.ListViolations(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("violation history: %w", err)
	}
	return page, total, nil
}
