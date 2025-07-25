package api

import (
	"net/http"

	"dispatch-and-delivery/internal/api/middleware"
	"dispatch-and-delivery/internal/modules/logistics"
	"dispatch-and-delivery/internal/modules/order"
	"dispatch-and-delivery/internal/modules/user"

	"github.com/labstack/echo/v4"
)

// SetupRoutes sets up all the API endpoints for the application.
func SetupRoutes(
	e *echo.Echo,
	jwtSecretKey string,
	userHandler *user.Handler,
	orderHandler *order.Handler,
	logisticsHandler *logistics.Handler,
) {
	// Initialize the JWT authentication middleware
	authMiddleware := middleware.JWTMAuth(jwtSecretKey)
	// Initialize an Admin role authorization middleware
	// adminRequired := middleware.AdminRequired()

	// --- Public Routes ---
	e.GET("/", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "Welcome to Circuit: Proudly Provides Logistics as a Service!"})
	})

	authGroup := e.Group("/auth")
	{
		authGroup.POST("/signup", userHandler.Signup)
		authGroup.POST("/login", userHandler.Login)
		authGroup.POST("/activate", userHandler.ActivateAccount)
		authGroup.POST("/resend-activation", userHandler.ResendActivation)
		authGroup.POST("/request-password-reset", userHandler.RequestPasswordReset)
		authGroup.POST("/reset-password", userHandler.ResetPassword)
		authGroup.GET("/google/login", userHandler.GoogleLogin)
		authGroup.GET("/google/callback", userHandler.GoogleCallback)
	}

	// --- User (Customer) Routes ---
	profileGroup := e.Group("/profile", authMiddleware)
	{

		// User Profile & Addresses
		profileGroup.GET("", userHandler.GetProfile)
		profileGroup.PUT("", userHandler.UpdateProfile)
		profileGroup.GET("/addresses", userHandler.ListAddresses)
		profileGroup.POST("/addresses", userHandler.AddAddress)
		profileGroup.PUT("/addresses/:addressId", userHandler.UpdateAddress)
		profileGroup.DELETE("/addresses/:addressId", userHandler.DeleteAddress)
	}

	// --- Order Routes ---
	orderGroup := e.Group("/orders", authMiddleware)
	{
		orderGroup.POST("/quote", orderHandler.GetDeliveryQuote) // Get route options and prices
		orderGroup.POST("", orderHandler.CreateOrder)
		orderGroup.GET("", orderHandler.ListMyOrders)
		orderGroup.GET("", orderHandler.ListAllOrders)
		orderGroup.GET("/:orderId", orderHandler.GetOrderDetails)
		orderGroup.PUT("/:orderId/cancel", orderHandler.CancelOrder)
		orderGroup.POST("/:orderId/pay", orderHandler.ConfirmAndPay)
		orderGroup.POST("/:orderId/feedback", orderHandler.SubmitFeedback)
	}

	// --- Logistics & Tracking Routes ---
	logisticsGroup := e.Group("/logistics", authMiddleware)
	{
		logisticsGroup.GET("/fleet", logisticsHandler.GetFleet)
		logisticsGroup.PUT("/fleet/:machineId/status", logisticsHandler.SetMachineStatus)
		logisticsGroup.POST("/orders/quote", logisticsHandler.CalculateQuote)
		logisticsGroup.POST("/orders/:orderId/route", logisticsHandler.ComputeRoute)
		logisticsGroup.POST("/orders/:orderId/assign", logisticsHandler.ReassignOrder)
		logisticsGroup.POST("/orders/:orderId/track", logisticsHandler.ReportTracking)
		logisticsGroup.GET("orders/:orderId/track", logisticsHandler.GetTracking)
	}
}
