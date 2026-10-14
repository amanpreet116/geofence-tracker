// Command seed loads the San Francisco demo dataset through the public HTTP
// API (so it doubles as a smoke test). It is idempotent: geofences are
// matched by name, vehicles by vehicle_number (409 = already there), and
// alert rules by the API's duplicate check (409 = already there).
//
//	seed -base-url http://localhost:8080
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/demo"
)

func main() {
	baseURL := flag.String("base-url", envOr("API_URL", "http://localhost:8080"), "API base URL")
	wait := flag.Duration("wait", 30*time.Second, "how long to wait for /health before giving up")
	flag.Parse()

	ctx := context.Background()
	if err := run(ctx, demo.NewClient(*baseURL), *wait); err != nil {
		fmt.Fprintln(os.Stderr, "seed failed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, c *demo.Client, wait time.Duration) error {
	if err := waitHealthy(ctx, c, wait); err != nil {
		return err
	}
	fmt.Printf("Seeding %s\n\n", c.BaseURL)

	// Geofences: create those missing by name.
	geoIDs, err := c.GeofenceIDs(ctx)
	if err != nil {
		return fmt.Errorf("list geofences: %w", err)
	}
	fmt.Println("Geofences")
	for _, g := range demo.Geofences {
		if id, ok := geoIDs[g.Name]; ok {
			fmt.Printf("  = %-32s %-16s %s\n", g.Name, g.Category, id)
			continue
		}
		var resp struct {
			ID string `json:"id"`
		}
		body := map[string]any{"name": g.Name, "description": g.Description, "category": g.Category, "coordinates": g.Coordinates}
		if err := c.Do(ctx, http.MethodPost, "/geofences", body, &resp); err != nil {
			return fmt.Errorf("create geofence %q: %w", g.Name, err)
		}
		geoIDs[g.Name] = resp.ID
		fmt.Printf("  + %-32s %-16s %s\n", g.Name, g.Category, resp.ID)
	}

	// Vehicles: POST; 409 means it already exists.
	fmt.Println("Vehicles")
	created := map[string]string{}
	for _, v := range demo.Vehicles {
		var resp struct {
			ID string `json:"id"`
		}
		body := map[string]any{"vehicle_number": v.VehicleNumber, "driver_name": v.DriverName, "vehicle_type": v.VehicleType, "phone": v.Phone}
		err := c.Do(ctx, http.MethodPost, "/vehicles", body, &resp)
		if apiErr, ok := errors.AsType[*demo.APIError](err); ok && apiErr.Status == http.StatusConflict {
			continue // reported below with its id
		}
		if err != nil {
			return fmt.Errorf("create vehicle %s: %w", v.VehicleNumber, err)
		}
		created[v.VehicleNumber] = resp.ID
	}
	vehIDs, err := c.VehicleIDs(ctx)
	if err != nil {
		return fmt.Errorf("list vehicles: %w", err)
	}
	for _, v := range demo.Vehicles {
		mark := "="
		if _, ok := created[v.VehicleNumber]; ok {
			mark = "+"
		}
		fmt.Printf("  %s %-12s %-14s %-11s %s\n", mark, v.VehicleNumber, v.DriverName, v.VehicleType, vehIDs[v.VehicleNumber])
	}

	// Alert rules: POST; 409 means an identical active rule exists.
	fmt.Println("Alert rules")
	for _, r := range demo.Rules {
		body := map[string]any{"geofence_id": geoIDs[r.Geofence], "event_type": r.EventType}
		target := "all vehicles"
		if r.Vehicle != "" {
			body["vehicle_id"] = vehIDs[r.Vehicle]
			target = r.Vehicle
		}
		var resp struct {
			AlertID string `json:"alert_id"`
		}
		err := c.Do(ctx, http.MethodPost, "/alerts/configure", body, &resp)
		mark, id := "+", resp.AlertID
		if apiErr, ok := errors.AsType[*demo.APIError](err); ok && apiErr.Status == http.StatusConflict {
			mark, id, err = "=", "(exists)", nil
		}
		if err != nil {
			return fmt.Errorf("create rule %s/%s: %w", r.Geofence, target, err)
		}
		fmt.Printf("  %s %-32s %-13s %-6s %s\n", mark, r.Geofence, target, r.EventType, id)
	}

	fmt.Println("\n+ created, = already present. Try:")
	fmt.Printf("  simulator -base-url %s -vehicle CA-TRK-1001\n", c.BaseURL)
	return nil
}

func waitHealthy(ctx context.Context, c *demo.Client, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		err := c.Do(ctx, http.MethodGet, "/health", nil, nil)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("API not healthy at %s after %s: %w", c.BaseURL, wait, err)
		}
		time.Sleep(time.Second)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
