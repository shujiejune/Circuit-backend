package models

import (
	"database/sql"
	"time"
)

// Order represents a delivery order in the system.
type Order struct {
	ID               string         `json:"id"`
	UserID           string         `json:"user_id"`
	MachineID        sql.NullString `json:"machine_id,omitempty"`
	PickupAddressID  string         `json:"pickup_address_id"`
	DropoffAddressID string         `json:"dropoff_address_id"`
	PickupAddress    *Address       `json:"pickup_address,omitempty"`
	DropoffAddress   *Address       `json:"dropoff_address,omitempty"`
	Status           string         `json:"status"`
	ItemDescription  string         `json:"item_description"`
	ItemWeightKg     float64        `json:"item_weight_kg"`
	Cost             float64        `json:"cost"`
	FeedbackRating   *int32         `json:"feedback_rating,omitempty"`
	FeedbackComment  *string        `json:"feedback_comment,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
}

// CreateOrderRequest represents the data needed to create a new order from a chosen route option.
type CreateOrderRequest struct {
	RouteOptionID string `json:"route_option_id" validate:"required"`
	Items         []byte `json:"items" validate:"required"`
}

// AdminUpdateOrderRequest represents the data an admin can use to update an order.
type AdminUpdateOrderRequest struct {
	Status    *string `json:"status,omitempty" validate:"omitempty,oneof=PENDING_PAYMENT CANCELLED CONFIRMED IN_PROGRESS DELIVERED FAILED"`
	MachineID *string `json:"machine_id,omitempty"`
}

// PaymentRequest represents the data needed to pay for an order.
type PaymentRequest struct {
	PaymentMethodID string `json:"payment_method_id" validate:"required"`
}

// FeedbackRequest represents the data needed to submit feedback for an order.
type FeedbackRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Comment string `json:"comment,omitempty"`
} 