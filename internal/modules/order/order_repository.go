package order

import (
	"context"
	"database/sql"
	"dispatch-and-delivery/internal/models"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RepositoryInterface defines the contract for the order repository.
type RepositoryInterface interface {
	Create(ctx context.Context, userID string, req models.CreateOrderRequest, pickupAddressID, dropoffAddressID string) (*models.Order, error)
	FindByID(ctx context.Context, orderID string) (*models.Order, error)
	ListByUserID(ctx context.Context, userID string, page, limit int) ([]*models.Order, int, error)
	ListAll(ctx context.Context, page, limit int) ([]*models.Order, int, error)
	UpdateStatusForUser(ctx context.Context, orderID string, userID string, status string) error
	InsertAddress(ctx context.Context, addr *models.Address) (string, error)
}

// Repository implements the RepositoryInterface.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new order repository.
func NewRepository(db *pgxpool.Pool) RepositoryInterface {
	return &Repository{db: db}
}

// Create inserts a new order into the database.
func (r *Repository) Create(ctx context.Context, userID string, req models.CreateOrderRequest, pickupAddressID, dropoffAddressID string) (*models.Order, error) {
	query := `
		INSERT INTO orders (user_id, pickup_address_id, dropoff_address_id, status, item_length_cm, item_width_cm, item_height_cm, item_weight_kg, cost)
		VALUES ($1, $2, $3, 'PENDING_PAYMENT', $4, $5, $6, $7, $8)
		RETURNING id, user_id, machine_id, pickup_address_id, dropoff_address_id, status, item_length_cm, item_width_cm, item_height_cm, item_weight_kg, cost, created_at, updated_at`

	// For now, using default values for weight and cost
	// In a real implementation, these would come from the route option
	const defaultWeight = 1.0
	const defaultCost = 15.75

	row := r.db.QueryRow(ctx, query, userID, pickupAddressID, dropoffAddressID, req.ItemLengthCm, req.ItemWidthCm, req.ItemHeightCm, defaultWeight, defaultCost)
	order, err := r.scanOrder(row)
	if err != nil {
		return nil, fmt.Errorf("repository.CreateOrder: %w", err)
	}
	return order, nil
}

// scanOrder is a helper function to scan a row into an Order model.
func (r *Repository) scanOrder(row pgx.Row) (*models.Order, error) {
	var order models.Order
	var machineIDFromDB sql.NullString
	err := row.Scan(
		&order.ID,
		&order.UserID,
		&machineIDFromDB,
		&order.PickupAddressID,
		&order.DropoffAddressID,
		&order.Status,
		&order.ItemLengthCm,
		&order.ItemWidthCm,
		&order.ItemHeightCm,
		&order.ItemWeightKg,
		&order.Cost,
		&order.CreatedAt,
		&order.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to scan order: %w", err)
	}

	if machineIDFromDB.Valid {
		order.MachineID = &machineIDFromDB.String
	} else {
		order.MachineID = nil
	}

	// Fetch feedback for this order
	feedback, err := r.getFeedbackByOrderID(context.Background(), order.ID)
	if err == nil {
		order.Feedback = feedback
	}
	// If feedback not found, just leave as nil

	return &order, nil
}

// getFeedbackByOrderID fetches feedback for a given order ID
func (r *Repository) getFeedbackByOrderID(ctx context.Context, orderID string) (*models.Feedback, error) {
	query := `SELECT id, order_id, rating, comment, created_at, updated_at FROM feedback WHERE order_id = $1`
	row := r.db.QueryRow(ctx, query, orderID)
	var fb models.Feedback
	if err := row.Scan(&fb.ID, &fb.OrderID, &fb.Rating, &fb.Comment, &fb.CreatedAt, &fb.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No feedback for this order
		}
		return nil, err
	}
	return &fb, nil
}

func (r *Repository) getAddressByID(ctx context.Context, addressID string) (*models.Address, error) {
	query := `SELECT id, user_id, label, street_address, is_default, created_at, updated_at FROM addresses WHERE id = $1`
	row := r.db.QueryRow(ctx, query, addressID)
	var addr models.Address
	err := row.Scan(
		&addr.ID,
		&addr.UserID,
		&addr.Label,
		&addr.StreetAddress,
		&addr.IsDefault,
		&addr.CreatedAt,
		&addr.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &addr, nil
}

// InsertAddress inserts a new address into the database and returns its ID.
func (r *Repository) InsertAddress(ctx context.Context, addr *models.Address) (string, error) {
	query := `INSERT INTO addresses (user_id, label, street_address, is_default) VALUES ($1, $2, $3, $4) RETURNING id`
	var id string
	err := r.db.QueryRow(ctx, query, addr.UserID, addr.Label, addr.StreetAddress, addr.IsDefault).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("repository.InsertAddress: %w", err)
	}
	return id, nil
}

// FindByID retrieves a single order by its ID.
func (r *Repository) FindByID(ctx context.Context, orderID string) (*models.Order, error) {
	query := `
		SELECT id, user_id, machine_id, pickup_address_id, dropoff_address_id, status, item_length_cm, item_width_cm, item_height_cm, item_weight_kg, cost, created_at, updated_at
		FROM orders
		WHERE id = $1`
	row := r.db.QueryRow(ctx, query, orderID)
	order, err := r.scanOrder(row)
	if err != nil {
		return nil, fmt.Errorf("repository.FindByID: %w", err)
	}

	if order.PickupAddressID != "" {
		addr, err := r.getAddressByID(ctx, order.PickupAddressID)
		if err == nil {
			order.PickupAddress = addr
		}
	}
	// 查询并赋值 DropoffAddress
	if order.DropoffAddressID != "" {
		addr, err := r.getAddressByID(ctx, order.DropoffAddressID)
		if err == nil {
			order.DropoffAddress = addr
		}
	}

	return order, nil
}

// ListByUserID retrieves all orders for a specific user with pagination.
func (r *Repository) ListByUserID(ctx context.Context, userID string, page, limit int) ([]*models.Order, int, error) {
	offset := (page - 1) * limit
	query := `
		SELECT id, user_id, machine_id, pickup_address_id, dropoff_address_id, status, item_length_cm, item_width_cm, item_height_cm, item_weight_kg, cost, created_at, updated_at
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.ListByUserID.Query: %w", err)
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.ListByUserID.scanOrder: %w", err)
		}
		orders = append(orders, order)
	}

	var total int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1", userID).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.ListByUserID.Count: %w", err)
	}

	return orders, total, nil
}

// ListAll retrieves all orders in the system with pagination (for admin use).
func (r *Repository) ListAll(ctx context.Context, page, limit int) ([]*models.Order, int, error) {
	offset := (page - 1) * limit
	query := `
		SELECT id, user_id, machine_id, pickup_address_id, dropoff_address_id, status, item_length_cm, item_width_cm, item_height_cm, item_weight_kg, cost, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.ListAll.Query: %w", err)
	}
	defer rows.Close()

	var orders []*models.Order
	for rows.Next() {
		order, err := r.scanOrder(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("repository.ListAll.scanOrder: %w", err)
		}
		orders = append(orders, order)
	}

	var total int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*) FROM orders").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("repository.ListAll.Count: %w", err)
	}

	return orders, total, nil
}

// UpdateStatusForUser updates the status of an order for a specific user.
// This is used for actions like cancelling an order.
func (r *Repository) UpdateStatusForUser(ctx context.Context, orderID string, userID string, status string) error {
	query := `
		UPDATE orders
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND user_id = $3`

	cmdTag, err := r.db.Exec(ctx, query, status, orderID, userID)
	if err != nil {
		return fmt.Errorf("repository.UpdateStatusForUser: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return models.ErrNotFound // Order not found or not owned by the user
	}

	return nil
}
