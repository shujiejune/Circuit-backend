package payment

import (
	"context"
	"fmt"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/paymentintent"
)

// ServiceInterface defines the contract for a payment processing service.
type ServiceInterface interface {
	ProcessPayment(ctx context.Context, userID string, amount float64, paymentMethodID string) (string, error)
}

// StripeService is a real implementation using Stripe.
type StripeService struct {
	apiKey string
}

func NewStripeService(apiKey string) *StripeService {
	stripe.Key = apiKey
	return &StripeService{apiKey: apiKey}
}

// ProcessPayment creates and confirms a Stripe PaymentIntent.
func (s *StripeService) ProcessPayment(ctx context.Context, userID string, amount float64, paymentMethodID string) (string, error) {
	params := &stripe.PaymentIntentParams{
		Amount:        stripe.Int64(int64(amount * 100)), // Stripe uses cents
		Currency:      stripe.String(string(stripe.CurrencyUSD)),
		PaymentMethod: stripe.String(paymentMethodID),
		Confirm:       stripe.Bool(true),
	}
	pi, err := paymentintent.New(params)
	if err != nil {
		return "", fmt.Errorf("stripe payment failed: %w", err)
	}
	return pi.ID, nil
} 