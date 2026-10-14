// Package demo holds the San Francisco demo dataset and a small API client
// shared by cmd/seed and cmd/simulator. Coordinates are [lat, lng] (API
// order). It talks to the server only over HTTP, never to the database.
package demo

// Point is [latitude, longitude].
type Point [2]float64

type Geofence struct {
	Name        string
	Description string
	Category    string
	Coordinates []Point // closed ring
}

type Vehicle struct {
	VehicleNumber string
	DriverName    string
	VehicleType   string
	Phone         string
}

// Rule references a geofence by name and a vehicle by number ("" = all).
type Rule struct {
	Geofence  string
	Vehicle   string
	EventType string
}

func rect(south, west, north, east float64) []Point {
	return []Point{{south, west}, {north, west}, {north, east}, {south, east}, {south, west}}
}

const RestrictedZone = "Port Terminal Restricted Area"

// Geofences cover all four categories. Downtown is the assignment's example
// square; SoMa deliberately overlaps it to demonstrate overlapping zones.
var Geofences = []Geofence{
	{
		Name: "Downtown Delivery Zone", Description: "Main delivery area for downtown customers",
		Category: "delivery_zone", Coordinates: rect(37.7749, -122.4194, 37.7849, -122.4094),
	},
	{
		Name: "SoMa Warehouse District", Description: "Warehouses and loading docks south of Market",
		Category: "delivery_zone", Coordinates: rect(37.7720, -122.4120, 37.7800, -122.3950),
	},
	{
		Name: RestrictedZone, Description: "Secured container terminal — authorized vehicles only",
		Category: "restricted_zone",
		Coordinates: []Point{
			{37.7860, -122.3920}, {37.7915, -122.3905}, {37.7930, -122.3850},
			{37.7880, -122.3825}, {37.7845, -122.3870}, {37.7860, -122.3920},
		},
	},
	{
		Name: "Bay Bridge Toll Plaza", Description: "Eastbound toll collection on I-80",
		Category: "toll_zone", Coordinates: rect(37.8220, -122.3180, 37.8280, -122.3050),
	},
	{
		Name: "Mission District Customers", Description: "Residential customer drop-off area",
		Category: "customer_area", Coordinates: rect(37.7550, -122.4250, 37.7650, -122.4100),
	},
}

var Vehicles = []Vehicle{
	{VehicleNumber: "CA-TRK-1001", DriverName: "Maria Lopez", VehicleType: "truck", Phone: "+14155550101"},
	{VehicleNumber: "CA-VAN-2002", DriverName: "James Chen", VehicleType: "van", Phone: "+14155550102"},
	{VehicleNumber: "CA-CAR-3003", DriverName: "Aisha Khan", VehicleType: "car", Phone: "+14155550103"},
	{VehicleNumber: "CA-MC-4004", DriverName: "Diego Rivera", VehicleType: "motorcycle", Phone: "+14155550104"},
}

var Rules = []Rule{
	{Geofence: RestrictedZone, Vehicle: "", EventType: "both"}, // every vehicle, entry and exit
	{Geofence: "Downtown Delivery Zone", Vehicle: "CA-TRK-1001", EventType: "entry"},
	{Geofence: "Bay Bridge Toll Plaza", Vehicle: "", EventType: "entry"},
	{Geofence: "Mission District Customers", Vehicle: "CA-VAN-2002", EventType: "exit"},
}

// Route goes Mission → Downtown → SoMa → through the restricted port
// terminal → out toward the Bay Bridge. Driving it crosses every zone type
// except the toll plaza (use -route bridge for that).
var Route = []Point{
	{37.7600, -122.4180}, // inside Mission District Customers
	{37.7680, -122.4150}, // leaving Mission
	{37.7790, -122.4140}, // into Downtown Delivery Zone
	{37.7775, -122.4105}, // Downtown ∩ SoMa (overlap: lat 37.7749–37.7800, lng -122.4120 to -122.4094)
	{37.7760, -122.3980}, // SoMa only
	{37.7830, -122.3940}, // heading to the port
	{37.7880, -122.3875}, // inside the restricted terminal
	{37.7905, -122.3855}, // still inside
	{37.7960, -122.3800}, // out of the terminal
	{37.8000, -122.3700}, // onto the bridge approach
}

// BridgeRoute crosses the toll plaza.
var BridgeRoute = []Point{
	{37.8000, -122.3700},
	{37.8150, -122.3400},
	{37.8250, -122.3120}, // toll plaza
	{37.8300, -122.2950},
}

var Routes = map[string][]Point{"restricted": Route, "bridge": BridgeRoute}
