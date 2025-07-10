package models

import "time"

// Feedback represents feedback for an order.
type Feedback struct {
	ID        string    `json:"id"`
	OrderID   string    `json:"order_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
} 