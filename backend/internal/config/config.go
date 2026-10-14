// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration. It is populated once at startup.
type Config struct {
	Port           int
	DatabaseURL    string
	AllowedOrigins []string
	LogLevel       slog.Level

	// Used by later tasks (rate limiting and API key auth).
	RateLimitRPS   float64
	RateLimitBurst int
	APIKeys        []string
	AuthEnabled    bool
}

// Addr returns the listen address for the HTTP server.
func (c Config) Addr() string {
	return fmt.Sprintf(":%d", c.Port)
}

// LookupFunc matches os.LookupEnv; injectable for tests.
type LookupFunc func(key string) (string, bool)

// Load reads configuration from the process environment.
func Load() (Config, error) {
	return LoadFrom(os.LookupEnv)
}

// LoadFrom reads configuration using lookup. All problems are reported together.
func LoadFrom(lookup LookupFunc) (Config, error) {
	var (
		cfg  Config
		errs []error
	)
	get := func(key, def string) string {
		if v, ok := lookup(key); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		return def
	}

	port, err := strconv.Atoi(get("PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		errs = append(errs, fmt.Errorf("PORT must be an integer between 1 and 65535, got %q", get("PORT", "")))
	}
	cfg.Port = port

	cfg.DatabaseURL = get("DATABASE_URL", "")
	if cfg.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required (e.g. postgres://user:pass@localhost:5432/db?sslmode=disable)"))
	}

	cfg.AllowedOrigins = splitList(get("ALLOWED_ORIGINS", "*"))
	if len(cfg.AllowedOrigins) == 0 {
		errs = append(errs, errors.New(`ALLOWED_ORIGINS must list at least one origin or "*"`))
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(get("LOG_LEVEL", "info"))); err != nil {
		errs = append(errs, fmt.Errorf("LOG_LEVEL must be one of debug, info, warn, error: %w", err))
	}

	rps, err := strconv.ParseFloat(get("RATE_LIMIT_RPS", "10"), 64)
	if err != nil || rps <= 0 {
		errs = append(errs, fmt.Errorf("RATE_LIMIT_RPS must be a positive number, got %q", get("RATE_LIMIT_RPS", "")))
	}
	cfg.RateLimitRPS = rps

	burst, err := strconv.Atoi(get("RATE_LIMIT_BURST", "20"))
	if err != nil || burst < 1 {
		errs = append(errs, fmt.Errorf("RATE_LIMIT_BURST must be a positive integer, got %q", get("RATE_LIMIT_BURST", "")))
	}
	cfg.RateLimitBurst = burst

	cfg.APIKeys = splitList(get("API_KEYS", ""))

	authEnabled, err := strconv.ParseBool(get("AUTH_ENABLED", "false"))
	if err != nil {
		errs = append(errs, fmt.Errorf("AUTH_ENABLED must be a boolean, got %q", get("AUTH_ENABLED", "")))
	}
	cfg.AuthEnabled = authEnabled
	if cfg.AuthEnabled && len(cfg.APIKeys) == 0 {
		errs = append(errs, errors.New("API_KEYS must be set when AUTH_ENABLED=true"))
	}

	if len(errs) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %w", errors.Join(errs...))
	}
	return cfg, nil
}

func splitList(s string) []string {
	var out []string
	for part := range strings.SplitSeq(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
