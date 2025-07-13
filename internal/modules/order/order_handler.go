package order

import (
	"net/http"
	"strconv"

	"dispatch-and-delivery/internal/models"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for orders.
type Handler struct {
	svc      ServiceInterface
	validate *validator.Validate // For request body validation
}

// NewHandler creates a new order handler.
func NewHandler(svc ServiceInterface) *Handler {
	return &Handler{
		svc:      svc,
		validate: validator.New(),
	}
}

func (h *Handler) GetDeliveryQuote(c echo.Context) error {
	var req models.RouteRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Invalid request body"})
	}

	if err := h.validate.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	options, err := h.svc.GetDeliveryQuote(c.Request().Context(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to get delivery quotes"})
	}

	return c.JSON(http.StatusOK, options)
}

func (h *Handler) CreateOrder(c echo.Context) error {
	userID := c.Get("userID").(string)

	var req models.CreateOrderRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	order, err := h.svc.CreateOrder(c.Request().Context(), userID, req)
	if err != nil {
		// Handle specific service errors
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "Route option not found"})
		}
		c.Logger().Error("Handler.CreateOrder: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to create order"})
	}

	return c.JSON(http.StatusCreated, order)
}

func (h *Handler) ListMyOrders(c echo.Context) error {
	userID := c.Get("userID").(string)

	// Extract pagination parameters
	page := 1
	limit := 10
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	orders, total, err := h.svc.ListUserOrders(c.Request().Context(), userID, page, limit)
	if err != nil {
		c.Logger().Error("Handler.ListMyOrders: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to retrieve orders"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{"orders": orders, "total": total})
}

func (h *Handler) GetOrderDetails(c echo.Context) error {
	userID := c.Get("userID").(string)
	role := c.Get("userRole").(string)

	orderID := c.Param("orderId")

	order, err := h.svc.GetOrderDetails(c.Request().Context(), orderID, userID, role)
	if err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "Order not found"})
		}
		if err == models.ErrForbidden {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Access denied"})
		}
		c.Logger().Error("Handler.GetOrderDetails: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to retrieve order details"})
	}

	return c.JSON(http.StatusOK, order)
}

func (h *Handler) CancelOrder(c echo.Context) error {
	userID := c.Get("userID").(string)

	orderID := c.Param("orderId")

	if err := h.svc.CancelOrder(c.Request().Context(), orderID, userID); err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "Order not found"})
		}
		if err == models.ErrForbidden {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Cannot cancel this order"})
		}
		c.Logger().Error("Handler.CancelOrder: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to cancel order"})
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) ConfirmAndPay(c echo.Context) error {
	userID := c.Get("userID").(string)

	orderID := c.Param("orderId")

	var req models.PaymentRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	order, err := h.svc.ConfirmAndPay(c.Request().Context(), userID, orderID, req)
	if err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "Order not found"})
		}
		if err == models.ErrForbidden {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Cannot pay for this order"})
		}
		c.Logger().Error("Handler.ConfirmAndPay: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to process payment"})
	}

	return c.JSON(http.StatusOK, order)
}

func (h *Handler) SubmitFeedback(c echo.Context) error {
	userID := c.Get("userID").(string)

	orderID := c.Param("orderId")

	var req models.FeedbackRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Invalid request body"})
	}
	if err := h.validate.Struct(req); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{Message: "Validation failed: " + err.Error()})
	}

	if err := h.svc.SubmitFeedback(c.Request().Context(), userID, orderID, req); err != nil {
		if err == models.ErrNotFound {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{Message: "Order not found"})
		}
		if err == models.ErrForbidden {
			return c.JSON(http.StatusForbidden, models.ErrorResponse{Message: "Cannot submit feedback for this order"})
		}
		c.Logger().Error("Handler.SubmitFeedback: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to submit feedback"})
	}

	return c.NoContent(http.StatusAccepted)
}

func (h *Handler) ListAllOrders(c echo.Context) error {
	// Role check is done in middleware
	page := 1
	limit := 10
	if pageStr := c.QueryParam("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr := c.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	orders, total, err := h.svc.ListAllOrders(c.Request().Context(), page, limit)
	if err != nil {
		c.Logger().Error("Handler.ListAllOrders: ", err)
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{Message: "Failed to list all orders"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"orders": orders, "total": total})
}
