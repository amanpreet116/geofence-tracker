// Command simulator drives vehicles along a waypoint route, posting to
// POST /vehicles/location every -interval. Distance per tick is
// speed × interval, interpolated along the route, so -speed acts as a
// time-lapse factor for demos.
//
//	simulator -vehicle CA-TRK-1001                      # one vehicle, restricted route
//	simulator -vehicle CA-TRK-1001,CA-VAN-2002 -stagger 5s
//	simulator -vehicle veh_abc123 -route bridge -speed 120 -interval 2s -loop
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amanpreet116/geofence-tracker/backend/internal/demo"
)

type options struct {
	baseURL  string
	vehicles []string
	route    []demo.Point
	interval time.Duration
	speedKmh float64
	stagger  time.Duration
	loop     bool
}

func main() {
	var o options
	var vehicles, route string
	flag.StringVar(&o.baseURL, "base-url", envOr("API_URL", "http://localhost:8080"), "API base URL")
	flag.StringVar(&vehicles, "vehicle", "CA-TRK-1001", "comma-separated vehicle ids (veh_...) or vehicle numbers")
	flag.StringVar(&route, "route", "restricted", "route: restricted (through the port terminal) or bridge (toll plaza)")
	flag.DurationVar(&o.interval, "interval", time.Second, "time between location updates")
	flag.Float64Var(&o.speedKmh, "speed", 250, "simulated speed in km/h (high values = time-lapse)")
	flag.DurationVar(&o.stagger, "stagger", 3*time.Second, "start delay between vehicles")
	flag.BoolVar(&o.loop, "loop", false, "drive the route back and forth until interrupted")
	flag.Parse()

	var ok bool
	if o.route, ok = demo.Routes[route]; !ok {
		fmt.Fprintf(os.Stderr, "unknown route %q (restricted, bridge)\n", route)
		os.Exit(2)
	}
	for v := range strings.SplitSeq(vehicles, ",") {
		if v = strings.TrimSpace(v); v != "" {
			o.vehicles = append(o.vehicles, v)
		}
	}
	if len(o.vehicles) == 0 || o.interval <= 0 || o.speedKmh <= 0 {
		fmt.Fprintln(os.Stderr, "need at least one -vehicle, and positive -interval and -speed")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, demo.NewClient(o.baseURL), o); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "simulator:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, c *demo.Client, o options) error {
	ids, err := resolveVehicles(ctx, c, o.vehicles)
	if err != nil {
		return err
	}
	step := o.speedKmh * 1000 / 3600 * o.interval.Seconds() // metres per tick
	path := densify(o.route, step)
	fmt.Printf("Route: %.2f km, %d updates per pass every %s (%.0f m/tick at %.0f km/h)\n",
		routeLength(o.route)/1000, len(path), o.interval, step, o.speedKmh)

	var wg sync.WaitGroup
	for i, label := range o.vehicles {
		wg.Go(func() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(i) * o.stagger):
			}
			drive(ctx, c, label, ids[label], path, o)
		})
	}
	wg.Wait()
	return ctx.Err()
}

func resolveVehicles(ctx context.Context, c *demo.Client, labels []string) (map[string]string, error) {
	byNumber, err := c.VehicleIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list vehicles (is the API up and seeded?): %w", err)
	}
	ids := map[string]string{}
	for _, l := range labels {
		switch {
		case strings.HasPrefix(l, "veh_"):
			ids[l] = l
		case byNumber[strings.ToUpper(l)] != "":
			ids[l] = byNumber[strings.ToUpper(l)]
		default:
			return nil, fmt.Errorf("vehicle %q not found (run the seed first)", l)
		}
	}
	return ids, nil
}

type locationResponse struct {
	CurrentGeofences []struct {
		GeofenceName string `json:"geofence_name"`
	} `json:"current_geofences"`
}

func drive(ctx context.Context, c *demo.Client, label, id string, path []demo.Point, o options) {
	ticker := time.NewTicker(o.interval)
	defer ticker.Stop()
	prev := ""
	for pass := 0; ; pass++ {
		points := path
		if pass%2 == 1 { // drive back on odd passes
			points = reversed(path)
		}
		for i, p := range points {
			var resp locationResponse
			err := c.Do(ctx, http.MethodPost, "/vehicles/location", map[string]any{
				"vehicle_id": id, "latitude": round6(p[0]), "longitude": round6(p[1]),
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}, &resp)
			switch {
			case errors.Is(err, context.Canceled):
				return
			case err != nil:
				fmt.Printf("%s %-12s update %3d/%d  ERROR %v\n", clock(), label, i+1, len(points), err)
			default:
				names := make([]string, 0, len(resp.CurrentGeofences))
				for _, g := range resp.CurrentGeofences {
					names = append(names, g.GeofenceName)
				}
				now := strings.Join(names, ", ")
				marker := " "
				if now != prev {
					marker = "*" // geofence membership changed on this tick
				}
				if now == "" {
					now = "-"
				}
				fmt.Printf("%s %-12s update %3d/%d  [%9.6f, %11.6f] %s inside: %s\n",
					clock(), label, i+1, len(points), p[0], p[1], marker, now)
				prev = strings.Join(names, ", ")
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
		if !o.loop {
			fmt.Printf("%s %-12s route complete\n", clock(), label)
			return
		}
	}
}

// densify returns points every stepM metres along route (plus the end).
func densify(route []demo.Point, stepM float64) []demo.Point {
	out := []demo.Point{route[0]}
	carry := 0.0
	for i := 1; i < len(route); i++ {
		a, b := route[i-1], route[i]
		seg := haversine(a, b)
		for d := stepM - carry; d < seg; d += stepM {
			t := d / seg
			out = append(out, demo.Point{a[0] + (b[0]-a[0])*t, a[1] + (b[1]-a[1])*t})
		}
		carry = math.Mod(carry+seg, stepM)
	}
	if last := route[len(route)-1]; out[len(out)-1] != last {
		out = append(out, last)
	}
	return out
}

func routeLength(route []demo.Point) float64 {
	total := 0.0
	for i := 1; i < len(route); i++ {
		total += haversine(route[i-1], route[i])
	}
	return total
}

func haversine(a, b demo.Point) float64 {
	const r = 6371000.0
	lat1, lat2 := a[0]*math.Pi/180, b[0]*math.Pi/180
	dLat, dLng := lat2-lat1, (b[1]-a[1])*math.Pi/180
	h := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLng/2)*math.Sin(dLng/2)
	return 2 * r * math.Asin(math.Sqrt(h))
}

func reversed(p []demo.Point) []demo.Point {
	out := make([]demo.Point, len(p))
	for i := range p {
		out[i] = p[len(p)-1-i]
	}
	return out
}

func round6(f float64) float64 { return math.Round(f*1e6) / 1e6 }

func clock() string { return time.Now().Format("15:04:05") }

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
