// Package models defines the data structures used in the delivery system.
// It includes the Machine model and request structures for updating machine status.
package models

import "time"

// MachineType defines the available machine categories.
const (
	MachineTypeDrone = "DRONE"
	MachineTypeRobot = "ROBOT"
)

// Machine status constants used throughout the application.
const (
	StatusIdle        = "IDLE"
	StatusInTransit   = "IN_TRANSIT"
	StatusCharging    = "CHARGING"
	StatusMaintenance = "MAINTENANCE"
)

// Machine represents a delivery machine such as a drone or ground robot.
type Machine struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Status       string    `json:"status"`
	Latitude     float64   `json:"latitude"`
	Longitude    float64   `json:"longitude"`
	BatteryLevel int       `json:"battery_level"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MachineStatusUpdateRequest contains fields for updating a machine's
// status and current location.
type MachineStatusUpdateRequest struct {
	Status    string  `json:"status"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}
