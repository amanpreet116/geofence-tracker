package config

import (
	"log/slog"
	"slices"
	"strings"
	"testing"
)

func env(m map[string]string) LookupFunc {
	return func(k string) (string, bool) {
		v, ok := m[k]
		return v, ok
	}
}

func TestLoadFrom(t *testing.T) {
	const dsn = "postgres://u:p@localhost:5432/db"

	tests := []struct {
		name    string
		env     map[string]string
		wantErr []string
		check   func(t *testing.T, c Config)
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": dsn},
			check: func(t *testing.T, c Config) {
				if c.Port != 8080 || c.LogLevel != slog.LevelInfo || c.AuthEnabled ||
					c.RateLimitRPS != 10 || c.RateLimitBurst != 20 ||
					!slices.Equal(c.AllowedOrigins, []string{"*"}) {
					t.Errorf("unexpected defaults: %+v", c)
				}
			},
		},
		{
			name: "overrides and list trimming",
			env: map[string]string{
				"DATABASE_URL":    dsn,
				"PORT":            "9000",
				"ALLOWED_ORIGINS": " http://a.com , ,http://b.com ",
				"LOG_LEVEL":       "DEBUG",
				"AUTH_ENABLED":    "true",
				"API_KEYS":        "k1,k2",
			},
			check: func(t *testing.T, c Config) {
				if c.Port != 9000 || c.LogLevel != slog.LevelDebug || !c.AuthEnabled {
					t.Errorf("unexpected config: %+v", c)
				}
				if !slices.Equal(c.AllowedOrigins, []string{"http://a.com", "http://b.com"}) {
					t.Errorf("origins = %v", c.AllowedOrigins)
				}
				if !slices.Equal(c.APIKeys, []string{"k1", "k2"}) {
					t.Errorf("api keys = %v", c.APIKeys)
				}
			},
		},
		{
			name:    "missing database url",
			env:     map[string]string{},
			wantErr: []string{"DATABASE_URL is required"},
		},
		{
			name: "multiple invalid values reported together",
			env: map[string]string{
				"DATABASE_URL":     dsn,
				"PORT":             "abc",
				"LOG_LEVEL":        "loud",
				"RATE_LIMIT_RPS":   "-1",
				"RATE_LIMIT_BURST": "0",
				"AUTH_ENABLED":     "maybe",
			},
			wantErr: []string{"PORT", "LOG_LEVEL", "RATE_LIMIT_RPS", "RATE_LIMIT_BURST", "AUTH_ENABLED"},
		},
		{
			name:    "auth enabled without keys",
			env:     map[string]string{"DATABASE_URL": dsn, "AUTH_ENABLED": "true"},
			wantErr: []string{"API_KEYS must be set"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadFrom(env(tt.env))
			if len(tt.wantErr) > 0 {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				for _, want := range tt.wantErr {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not mention %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}
