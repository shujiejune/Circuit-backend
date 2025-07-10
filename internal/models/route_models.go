package models

import "time"

// RouteRequest is the input from the user to get route options.
type RouteRequest struct {
	PickupLocation   string  `json:"pickup_location" validate:"required"`
	DeliveryLocation string  `json:"delivery_location" validate:"required"`
	ItemLengthCm     float64 `json:"item_length_cm" validate:"required,gt=0"`
	ItemWidthCm      float64 `json:"item_width_cm" validate:"required,gt=0"`
	ItemHeightCm     float64 `json:"item_height_cm" validate:"required,gt=0"`
}

// RouteOption represents a single routing option with a price and estimated duration.
type RouteOption struct {
	ID                string        `json:"id"`
	PickupLocation    Address       `json:"pickup_location"`
	DeliveryLocation  Address       `json:"delivery_location"`
	Price             float64       `json:"price"`
	EstimatedDuration time.Duration `json:"estimated_duration"` // in nanoseconds
}

// Machine represents a delivery machine (robot, drone, etc.)
type Machine struct {
	ID             string    `json:"id"`
	MachineType    string    `json:"machine_type"`
	MachineStatus  string    `json:"machine_status"`
	CurrentLocation string   `json:"current_location"`
	BatteryLevel   int       `json:"battery_level"`
	UpdatedAt      time.Time `json:"updated_at"`
} 