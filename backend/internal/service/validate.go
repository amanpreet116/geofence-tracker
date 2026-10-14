// Package service holds business logic: input validation, geofence
// entry/exit detection and alert matching. It depends on repository
// interfaces it declares itself.
package service

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

const (
	maxGeofenceNameLen  = 200
	minPolygonPoints    = 4    // 3 unique + closing point
	maxPolygonPoints    = 1000 // bounds request size and point-in-polygon cost
	minUniquePolygonPts = 3
)

// fieldErrors collects messages per JSON field; several messages for the
// same field are joined with "; ".
type fieldErrors map[string]string

func (f fieldErrors) add(field, msg string) {
	if prev, ok := f[field]; ok {
		f[field] = prev + "; " + msg
		return
	}
	f[field] = msg
}

func (f fieldErrors) err(message string) error {
	if len(f) == 0 {
		return nil
	}
	return model.NewValidationError(message, f)
}

// CreateGeofenceInput is the raw POST /geofences payload. Coordinates stay
// [][]float64 so pair length can be validated.
type CreateGeofenceInput struct {
	Name        string
	Description string
	Coordinates [][]float64
	Category    string
}

// ValidateCreateGeofence checks everything that does not need the database
// and returns a normalized geofence (trimmed strings, typed coordinates).
// All field errors are reported together.
func ValidateCreateGeofence(in CreateGeofenceInput) (model.Geofence, error) {
	errs := fieldErrors{}

	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		errs.add("name", "is required")
	case utf8.RuneCountInString(name) > maxGeofenceNameLen:
		errs.add("name", fmt.Sprintf("must be at most %d characters", maxGeofenceNameLen))
	}

	category := model.Category(strings.TrimSpace(in.Category))
	if msg := categoryProblem(category); msg != "" {
		errs.add("category", msg)
	}

	ring := validateRing(in.Coordinates, errs)

	if err := errs.err("invalid geofence"); err != nil {
		return model.Geofence{}, err
	}
	return model.Geofence{
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Category:    category,
		Coordinates: ring,
	}, nil
}

// ValidateCategoryFilter validates the optional ?category= query parameter.
// An empty value means "no filter".
func ValidateCategoryFilter(raw string) (model.Category, error) {
	c := model.Category(strings.TrimSpace(raw))
	if c == "" {
		return "", nil
	}
	if msg := categoryProblem(c); msg != "" {
		return "", fieldErrors{"category": msg}.err("invalid query parameter")
	}
	return c, nil
}

func categoryProblem(c model.Category) string {
	if c == "" {
		return "is required"
	}
	if !c.Valid() {
		names := make([]string, len(model.Categories))
		for i, v := range model.Categories {
			names[i] = string(v)
		}
		return "must be one of " + strings.Join(names, ", ")
	}
	return ""
}

func validateRing(coords [][]float64, errs fieldErrors) []model.LatLng {
	if len(coords) == 0 {
		errs.add("coordinates", "is required")
		return nil
	}
	if len(coords) > maxPolygonPoints {
		errs.add("coordinates", fmt.Sprintf("must contain at most %d points", maxPolygonPoints))
		return nil
	}

	ring := make([]model.LatLng, 0, len(coords))
	pairsOK := true
	for i, pair := range coords {
		key := fmt.Sprintf("coordinates[%d]", i)
		if len(pair) != 2 {
			errs.add(key, "must be a [latitude, longitude] pair")
			pairsOK = false
			continue
		}
		lat, lng := pair[0], pair[1]
		if lat < -90 || lat > 90 {
			errs.add(key, "latitude must be between -90 and 90")
			pairsOK = false
		}
		if lng < -180 || lng > 180 {
			errs.add(key, "longitude must be between -180 and 180")
			pairsOK = false
		}
		ring = append(ring, model.LatLng{Lat: lat, Lng: lng})
	}

	if len(coords) < minPolygonPoints {
		errs.add("coordinates", fmt.Sprintf("must contain at least %d points (3 unique points plus the closing point)", minPolygonPoints))
		return nil
	}
	if !pairsOK {
		return nil
	}
	if ring[0] != ring[len(ring)-1] {
		errs.add("coordinates", "polygon must be closed: first and last points must be identical")
	}
	if countUnique(ring) < minUniquePolygonPts {
		errs.add("coordinates", fmt.Sprintf("must contain at least %d unique points", minUniquePolygonPts))
	}
	if crossesAntimeridian(ring) {
		errs.add("coordinates", "polygon must not cross the antimeridian (±180° longitude); split it into two geofences")
	}
	return ring
}

// crossesAntimeridian reports an edge spanning more than 180° of longitude
// between two interior longitudes (e.g. 179 → -179). Geometry is planar in
// degrees, so such an edge would be drawn the long way round the globe, not
// across the date line the user meant. Edges ending exactly on ±180 are
// deliberate map-edge spans (e.g. a whole-world box) and are allowed.
func crossesAntimeridian(ring []model.LatLng) bool {
	for i := 1; i < len(ring); i++ {
		a, b := ring[i-1].Lng, ring[i].Lng
		if (b-a > 180 || a-b > 180) && math.Abs(a) != 180 && math.Abs(b) != 180 {
			return true
		}
	}
	return false
}

func countUnique(ring []model.LatLng) int {
	seen := make(map[model.LatLng]struct{}, len(ring))
	for _, p := range ring {
		seen[p] = struct{}{}
	}
	return len(seen)
}
