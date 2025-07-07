package models

import "time"

// Strategy constants for different routing modes.
const (
	FastestStrategy  = "FASTEST"
	CheapestStrategy = "CHEAPEST"
)
// RouteRequest is the input from the user to get route options.
type RouteRequest struct {
	// When provided, PickupLocation and DeliveryLocation can be omitted and
	// the service will load addresses from the order record.
	OrderID          string    `json:"order_id,omitempty"`
	PickupLocation   string    `json:"pickup_location,omitempty"`
	DeliveryLocation string    `json:"delivery_location,omitempty"`
	RequestedTime    time.Time `json:"requested_time,omitempty"`
}

// RouteOption represents a single routing option with a price and estimated duration.
type RouteOption struct {
	ID               string `json:"id"`
	PickupLocation   string `json:"pickup_location,omitempty"`
	DeliveryLocation string `json:"delivery_location,omitempty"`
	// Additional information returned by the logistics module
	Polyline        string  `json:"polyline,omitempty"`
	DistanceMeters  int     `json:"distance_meters,omitempty"`
	DurationSeconds int     `json:"duration_seconds,omitempty"`
	Strategy        string  `json:"strategy,omitempty"`
	MachineType     string  `json:"machine_type,omitempty"`
	EstimatedCost   float64 `json:"estimated_cost,omitempty"`

	// Legacy pricing fields kept for compatibility with the order module
	Price             float64       `json:"price,omitempty"`
	EstimatedDuration time.Duration `json:"estimated_duration,omitempty"`
} 

// Route represents a persisted route calculated for an order.
// It stores the encoded polyline returned by Google Maps Directions API
// along with distance and duration metrics.  This data can later be used
// for tracking or re-displaying the route to users.
type Route struct {
	ID              string    `json:"id"`
	OrderID         string    `json:"order_id"`
	Polyline        string    `json:"polyline"`
	DistanceMeters  int       `json:"distance_meters"`
	DurationSeconds int       `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}