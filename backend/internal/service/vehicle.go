package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

const (
	maxDriverNameLen  = 100
	maxVehicleTypeLen = 50
)

var (
	// 3-20 chars of A-Z, 0-9, hyphen or space; must start and end alphanumeric.
	vehicleNumberPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9 -]{1,18}[A-Z0-9]$`)
	// Optional leading +, then 7-15 digits (E.164 allows at most 15).
	phonePattern = regexp.MustCompile(`^\+?[0-9]{7,15}$`)
	// Separators people commonly type in phone numbers.
	phoneSeparators = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", ".", "")
)

// CreateVehicleInput is the raw POST /vehicles payload.
type CreateVehicleInput struct {
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
}

// ValidateCreateVehicle trims and normalizes in and reports all field
// errors together. vehicle_number is upper-cased with whitespace runs
// collapsed, so "ka-01  ab" and "KA-01 AB" are the same vehicle.
func ValidateCreateVehicle(in CreateVehicleInput) (model.Vehicle, error) {
	errs := fieldErrors{}

	number := strings.ToUpper(strings.Join(strings.Fields(in.VehicleNumber), " "))
	switch {
	case number == "":
		errs.add("vehicle_number", "is required")
	case !vehicleNumberPattern.MatchString(number):
		errs.add("vehicle_number", "must be 3-20 characters of letters, digits, hyphens or spaces, starting and ending with a letter or digit")
	}

	driver := strings.TrimSpace(in.DriverName)
	requireText(errs, "driver_name", driver, maxDriverNameLen)

	vehicleType := strings.TrimSpace(in.VehicleType)
	requireText(errs, "vehicle_type", vehicleType, maxVehicleTypeLen)

	phone := strings.TrimSpace(in.Phone)
	switch {
	case phone == "":
		errs.add("phone", "is required")
	case !phonePattern.MatchString(phoneSeparators.Replace(phone)):
		errs.add("phone", "must be 7-15 digits with an optional leading +")
	}

	if err := errs.err("invalid vehicle"); err != nil {
		return model.Vehicle{}, err
	}
	return model.Vehicle{
		VehicleNumber: number,
		DriverName:    driver,
		VehicleType:   vehicleType,
		Phone:         phone,
	}, nil
}

func requireText(errs fieldErrors, field, value string, maxLen int) {
	switch {
	case value == "":
		errs.add(field, "is required")
	case utf8.RuneCountInString(value) > maxLen:
		errs.add(field, fmt.Sprintf("must be at most %d characters", maxLen))
	}
}

// VehicleRepository is the persistence the vehicle service needs.
type VehicleRepository interface {
	// CreateVehicle returns a model.ErrConflict error if the vehicle_number exists.
	CreateVehicle(ctx context.Context, v *model.Vehicle) error
	ListVehicles(ctx context.Context) ([]model.Vehicle, error)
	ListFleetLocations(ctx context.Context) ([]model.FleetVehicle, error)
}

type VehicleService struct {
	repo VehicleRepository
}

func NewVehicleService(repo VehicleRepository) *VehicleService {
	return &VehicleService{repo: repo}
}

// Create validates and registers a vehicle.
func (s *VehicleService) Create(ctx context.Context, in CreateVehicleInput) (model.Vehicle, error) {
	v, err := ValidateCreateVehicle(in)
	if err != nil {
		return model.Vehicle{}, err
	}
	v.ID = model.NewID(model.PrefixVehicle)
	v.Status = model.VehicleStatusActive
	if err := s.repo.CreateVehicle(ctx, &v); err != nil {
		return model.Vehicle{}, fmt.Errorf("create vehicle: %w", err)
	}
	return v, nil
}

// List returns all vehicles, newest first.
func (s *VehicleService) List(ctx context.Context) ([]model.Vehicle, error) {
	vehicles, err := s.repo.ListVehicles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vehicles: %w", err)
	}
	return vehicles, nil
}

// FleetLocations returns every vehicle with its last position and current
// geofences in one call (for the frontend map).
func (s *VehicleService) FleetLocations(ctx context.Context) ([]model.FleetVehicle, error) {
	fleet, err := s.repo.ListFleetLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list fleet locations: %w", err)
	}
	return fleet, nil
}
