package model

import (
	"regexp"
	"testing"
)

func TestNewID(t *testing.T) {
	prefixes := []string{PrefixGeofence, PrefixVehicle, PrefixAlert, PrefixViolation, PrefixEvent, PrefixRequest}
	for _, p := range prefixes {
		t.Run(p, func(t *testing.T) {
			re := regexp.MustCompile("^" + p + "[a-z2-7]{12}$")
			seen := map[string]bool{}
			for range 1000 {
				id := NewID(p)
				if !re.MatchString(id) {
					t.Fatalf("id %q does not match %s", id, re)
				}
				if seen[id] {
					t.Fatalf("duplicate id %q", id)
				}
				seen[id] = true
			}
		})
	}
}
