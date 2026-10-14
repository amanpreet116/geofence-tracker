// Package repository contains hand-written SQL against PostgreSQL/PostGIS.
//
// COORDINATE ORDER: the API speaks [latitude, longitude]; PostGIS (and WKT,
// GeoJSON) speak (x, y) = (longitude, latitude). This file is the ONLY place
// in the codebase that converts between the two. Everything above the
// repository uses model.LatLng; every query below feeds PostGIS through the
// helpers here.
package repository

import (
	"fmt"
	"regexp"

	"github.com/amanpreet116/geofence-tracker/backend/internal/model"
)

// splitRing converts an API-order ring into parallel PostGIS-order arrays
// (x = longitude, y = latitude) for polygonSQL. Floats are sent in binary,
// so values are stored exactly (no decimal formatting round trip).
func splitRing(ring []model.LatLng) (lngs, lats []float64) {
	lngs = make([]float64, len(ring))
	lats = make([]float64, len(ring))
	for i, p := range ring {
		lngs[i], lats[i] = p.Lng, p.Lat
	}
	return lngs, lats
}

// pointArgs returns p in PostGIS order (x = longitude, y = latitude) for
// pointSQL.
func pointArgs(p model.LatLng) (lng, lat float64) {
	return p.Lng, p.Lat
}

// pointSQL returns a SQL expression for an SRID 4326 point from two float8
// parameters: lngPlaceholder is x, latPlaceholder is y.
func pointSQL(lngPlaceholder, latPlaceholder string) string {
	return fmt.Sprintf(`ST_SetSRID(ST_MakePoint(%s::float8, %s::float8), 4326)`, lngPlaceholder, latPlaceholder)
}

// polygonSQL returns a SQL expression building an SRID 4326 polygon from
// two float8[] parameters: lngPlaceholder holds x values, latPlaceholder y
// values, in ring order.
func polygonSQL(lngPlaceholder, latPlaceholder string) string {
	return fmt.Sprintf(`ST_SetSRID(ST_MakePolygon(ST_MakeLine(ARRAY(
		SELECT ST_MakePoint(p.x, p.y)
		FROM unnest(%s::float8[], %s::float8[]) WITH ORDINALITY AS p(x, y, ord)
		ORDER BY p.ord
	))), 4326)`, lngPlaceholder, latPlaceholder)
}

// ringPointsSQL selects the exterior ring of a polygon column as a 2-D
// float8 array of PostGIS-order [x, y] pairs; decode with joinRing.
// ST_X/ST_Y return the stored doubles exactly (GeoJSON output would round).
func ringPointsSQL(column string) string {
	return fmt.Sprintf(`(SELECT array_agg(ARRAY[ST_X(dp.geom), ST_Y(dp.geom)] ORDER BY dp.path)
		FROM ST_DumpPoints(%s) AS dp)`, column)
}

// joinRing converts PostGIS-order [x, y] pairs back to API-order LatLng.
func joinRing(xy [][]float64) ([]model.LatLng, error) {
	ring := make([]model.LatLng, len(xy))
	for i, p := range xy {
		if len(p) != 2 {
			return nil, fmt.Errorf("ring point %d has %d dimensions, want 2", i, len(p))
		}
		ring[i] = model.LatLng{Lat: p[1], Lng: p[0]}
	}
	return ring, nil
}

// reasonLocation matches the "[x y]" location PostGIS appends to
// ST_IsValidReason output, e.g. "Self-intersection[-122.41 37.77]".
var reasonLocation = regexp.MustCompile(`^(.*?)\s*\[(\S+) (\S+)\]$`)

// apiOrderReason rewrites a PostGIS validity reason so the location reads in
// API order: "Self-intersection at [37.77, -122.41]".
func apiOrderReason(reason string) string {
	m := reasonLocation.FindStringSubmatch(reason)
	if m == nil {
		return reason
	}
	return fmt.Sprintf("%s at [%s, %s]", m[1], m[3], m[2])
}
