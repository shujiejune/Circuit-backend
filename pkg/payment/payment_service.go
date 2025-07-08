package payment

import (
	"context"
	"fmt"
)

// ServiceInterface defines the contract for a payment processing service.
type ServiceInterface interface {
	ProcessPayment(ctx context.Context, userID string, amount float64, paymentMethodID string) (string, error)
}

// StripeService is a mock implementation using Stripe (replace with real SDK calls in production).
type StripeService struct {
	// Add Stripe client/config fields here if needed
}

func NewStripeService() *StripeService {
	return &StripeService{}
}

// ProcessPayment simulates a payment via Stripe.
func (s *StripeService) ProcessPayment(ctx context.Context, userID string, amount float64, paymentMethodID string) (string, error) {
	// Here you would call the real Stripe SDK, e.g.:
	// charge, err := stripeClient.Charges.New(...)
	// For now, just simulate success
	if amount <= 0 {
		return "", fmt.Errorf("invalid payment amount")
	}
	// Simulate a payment ID
	return "stripe_payment_id_123", nil
} 