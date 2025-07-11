package order

import (
	"context"
	"dispatch-and-delivery/internal/models"
	"fmt"
	"log"
	"sync"
)

// LogisticsServiceInterface defines the contract for the logistics service.
type LogisticsServiceInterface interface {
	CalculateRouteOptions(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error)
	AssignOrder(ctx context.Context, orderID, machineID string) (*models.Machine, error)
}

// ServiceInterface defines the contract for the order service.
type ServiceInterface interface {
	CreateOrder(ctx context.Context, userID string, req models.CreateOrderRequest) (*models.Order, error)
	GetOrderDetails(ctx context.Context, orderID string, userID string, role string) (*models.Order, error)
	ListUserOrders(ctx context.Context, userID string, page, limit int) ([]*models.Order, int, error)
	ListAllOrders(ctx context.Context, page, limit int) ([]*models.Order, int, error)
	CancelOrder(ctx context.Context, orderID string, userID string) error
	ConfirmAndPay(ctx context.Context, userID string, orderID string, req models.PaymentRequest) (*models.Order, error)
	SubmitFeedback(ctx context.Context, userID string, orderID string, req models.FeedbackRequest) error
	GetDeliveryQuote(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error)
}

// PaymentServiceInterface defines the contract for a payment processing service.
type PaymentServiceInterface interface {
	ProcessPayment(ctx context.Context, userID string, amount float64, paymentMethodID string) (string, error)
}


// Service implements the order service logic.
type Service struct {
	repo           RepositoryInterface
	// mapsService    MapsServiceInterface // For interacting with an external maps API. (remove)
	routeCache     map[string]*models.RouteOption // In-memory cache for route options
	routeCacheLock sync.RWMutex
	paymentService PaymentServiceInterface
	logisticsService LogisticsServiceInterface // Inject logistics service
}

// NewService creates a new order service.
func NewService(repo RepositoryInterface, /*mapsService MapsServiceInterface,*/ paymentService PaymentServiceInterface, logisticsService LogisticsServiceInterface) *Service {
	return &Service{
		repo:             repo,
		// mapsService:      mapsService, // remove
		routeCache:       make(map[string]*models.RouteOption),
		paymentService:   paymentService,
		logisticsService: logisticsService,
	}
}

// CreateOrder creates a new order based on a user's selected route option.
func (s *Service) CreateOrder(ctx context.Context, userID string, req models.CreateOrderRequest) (*models.Order, error) {
	s.routeCacheLock.RLock()
	routeOption, ok := s.routeCache[req.RouteOptionID]
	s.routeCacheLock.RUnlock()

	if !ok {
		return nil, models.ErrRouteOptionExpired
	}

	// Insert pickup and dropoff addresses, get their IDs
	pickupAddr := routeOption.PickupLocation
	pickupAddr.UserID = userID
	pickupID, err := s.repo.InsertAddress(ctx, &pickupAddr)
	if err != nil {
		return nil, fmt.Errorf("service.CreateOrder: failed to insert pickup address: %w", err)
	}
	dropoffAddr := routeOption.DeliveryLocation
	dropoffAddr.UserID = userID
	dropoffID, err := s.repo.InsertAddress(ctx, &dropoffAddr)
	if err != nil {
		return nil, fmt.Errorf("service.CreateOrder: failed to insert dropoff address: %w", err)
	}

	order, err := s.repo.Create(ctx, userID, req, pickupID, dropoffID)
	if err != nil {
		return nil, fmt.Errorf("service.CreateOrder: %w", err)
	}

	// Remove the route option from the cache after it has been used.
	s.routeCacheLock.Lock()
	delete(s.routeCache, req.RouteOptionID)
	s.routeCacheLock.Unlock()

	return order, nil
}

// GetOrderDetails retrieves a single order's details.
func (s *Service) GetOrderDetails(ctx context.Context, orderID string, userID string, role string) (*models.Order, error) {
	order, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("service.GetOrderDetails: %w", err)
	}

	// Security check: ensure the user requesting the order is the one who owns it.
	if order.UserID != userID {
		return nil, models.ErrNotFound // Return NotFound to avoid leaking information
	}

	return order, nil
}

// ListUserOrders retrieves all orders for a specific user.
func (s *Service) ListUserOrders(ctx context.Context, userID string, page, limit int) ([]*models.Order, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20 // Default/max limit
	}
	orders, total, err := s.repo.ListByUserID(ctx, userID, page, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("service.ListUserOrders: %w", err)
	}
	return orders, total, nil
}

// ListAllOrders lists all orders in the system.
func (s *Service) ListAllOrders(ctx context.Context, page, limit int) ([]*models.Order, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 50
	}
	return s.repo.ListAll(ctx, page, limit)
}

// CancelOrder cancels an order for a user.
func (s *Service) CancelOrder(ctx context.Context, orderID string, userID string) error {
	// First, retrieve the order to check its current status.
	order, err := s.GetOrderDetails(ctx, orderID, userID, "user") // This already checks ownership
	if err != nil {
		return err // Either not found or another DB error
	}

	// Business logic: an order can only be cancelled if it's in a 'PENDING_PAYMENT' state.
	if order.Status != "PENDING_PAYMENT" {
		return models.ErrOrderCannotBeCancelled
	}

	return s.repo.UpdateStatusForUser(ctx, orderID, userID, "CANCELLED")
}

// ConfirmAndPay confirms and pays for an order.
func (s *Service) ConfirmAndPay(ctx context.Context, userID string, orderID string, req models.PaymentRequest) (*models.Order, error) {
	// 1. Get the order details, ensuring it belongs to the user.
	order, err := s.GetOrderDetails(ctx, orderID, userID, "user")
	if err != nil {
		return nil, err // Handles not found or not authorized
	}

	// 2. Check if the order can be paid for.
	if order.Status != "PENDING_PAYMENT" {
		return nil, models.ErrOrderCannotBePaid
	}

	// 3. Process payment through the payment service.
	_, err = s.paymentService.ProcessPayment(ctx, userID, order.Cost, req.PaymentMethodID)
	if err != nil {
		return nil, fmt.Errorf("payment processing failed: %w", err)
	}

	// 4. Update order status to 'CONFIRMED' after successful payment.
	updateReq := models.AdminUpdateOrderRequest{
		Status: &[]string{"CONFIRMED"}[0],
	}
	updatedOrder, err := s.repo.Update(ctx, orderID, updateReq)
	if err != nil {
		// This is a critical error. The payment went through but we couldn't update our DB.
		log.Printf("CRITICAL: Payment processed for order %s but failed to update status: %v", orderID, err)
		return nil, fmt.Errorf("failed to update order status after successful payment: %w", err)
	}

	// 5. Call logisticsService.AssignOrder after payment and status update
	machineID := ""
	if updatedOrder.MachineID != nil {
		machineID = *updatedOrder.MachineID
	}
	_, err = s.logisticsService.AssignOrder(ctx, updatedOrder.ID, machineID)
	if err != nil {
		return nil, fmt.Errorf("failed to assign delivery after payment: %w", err)
	}

	return updatedOrder, nil
}

// SubmitFeedback allows a user to submit feedback for a completed order.
// Note: This functionality is not available in the current database schema
// as there are no feedback fields in the orders table.
func (s *Service) SubmitFeedback(ctx context.Context, userID string, orderID string, req models.FeedbackRequest) error {
	order, err := s.GetOrderDetails(ctx, orderID, userID, "user")
	if err != nil {
		return err
	}
	if order.Status != "DELIVERED" {
		return models.ErrCannotSubmitFeedback
	}
	if order.Feedback != nil {
		return models.ErrFeedbackAlreadySubmitted
	}
	return s.repo.InsertFeedback(ctx, orderID, req)
}

func (s *Service) GetDeliveryQuote(ctx context.Context, req models.RouteRequest) ([]models.RouteOption, error) {
	return s.logisticsService.CalculateRouteOptions(ctx, req)
}
