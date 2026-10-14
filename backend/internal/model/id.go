package model

import (
	"crypto/rand"
	"strings"
)

// ID prefixes required by the API contract.
const (
	PrefixGeofence  = "geo_"
	PrefixVehicle   = "veh_"
	PrefixAlert     = "alert_"
	PrefixViolation = "viol_"
	PrefixEvent     = "evt_"
	PrefixRequest   = "req_"
)

const idLength = 12 // 12 base32 chars = 60 bits of randomness.

// NewID returns prefix followed by 12 random lowercase base32 characters
// (a-z, 2-7), e.g. "geo_k3v9q2mfx7ab".
func NewID(prefix string) string {
	// rand.Text yields 26 uniformly random base32 chars from crypto/rand.
	return prefix + strings.ToLower(rand.Text()[:idLength])
}
