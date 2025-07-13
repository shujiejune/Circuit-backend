package models

import "time"

// Strategy constants for different routing modes.
const (
	FastestStrategy  = "FASTEST"
	CheapestStrategy = "CHEAPEST"
)

// Dimensions describes the package size in meters.
type Dimensions struct {
	Length float64 `json:"length_m" validate:"required,gt=0"`
	Width  float64 `json:"width_m" validate:"required,gt=0"`
	Height float64 `json:"height_m" validate:"required,gt=0"`
}

// RouteRequest is the input from the user to get route options.
type RouteRequest struct {
	// When provided, PickupLocation and DeliveryLocation can be omitted and
	// the service will load addresses from the order record.
	PickupLocation   Address    `json:"pickup_location"`
	DeliveryLocation Address    `json:"delivery_location"`
	WeightKG         float64    `json:"weight_kg"`
	Dimensions       Dimensions `json:"dimensions"`
	RequestedTime    time.Time  `json:"requested_time"`
	OrderID          string     `json:"order_id,omitempty"`
}

// RouteOption represents a single routing option with a price and estimated duration.
type RouteOption struct {
	ID                string        `json:"id"`
	PickupLocation    Address       `json:"pickup_location"`
	DeliveryLocation  Address       `json:"delivery_location"`
	Price             float64       `json:"price"`
	EstimatedDuration time.Duration `json:"estimated_duration"`
	Polyline          string        `json:"polyline,omitempty"`
	DistanceMeters    int           `json:"distance_meters,omitempty"`
	DurationSeconds   int           `json:"duration_seconds,omitempty"`
	Strategy          string        `json:"strategy,omitempty"`
	EstimatedCost     float64       `json:"estimated_cost,omitempty"`
	MachineType       string        `json:"machine_type,omitempty"`
}

// Route represents a persisted route calculated for an order.
type Route struct {
	ID              string    `json:"id"`
	OrderID         string    `json:"order_id"`
	Polyline        string    `json:"polyline"`
	DistanceMeters  int       `json:"distance_meters"`
	DurationSeconds int       `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}
