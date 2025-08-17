package example

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"obs-brutal/pkg/obsvbrutal"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
)

// UserHandler handles user-related endpoints
type UserHandler struct {
	logger obsvbrutal.Logger
}

// NewUserHandler creates new user handler
func NewUserHandler(logger obsvbrutal.Logger) *UserHandler {
	return &UserHandler{
		logger: logger.Mod("users"),
	}
}

// CreateUser creates a new user
func (h *UserHandler) CreateUser(c *gin.Context) {
	// Get request logger
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `json:"name" binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
		Phone    string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid user creation request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request data",
			"details": err.Error(),
		})
		return
	}

	// Log the request (sensitive data will be masked)
	reqLogger.
		F("email", req.Email).
		F("name", req.Name).
		F("password", req.Password). // Will be masked
		F("phone", req.Phone).       // Will be masked if Thai phone number
		Info("Creating new user")

	// Simulate user creation
	userID := fmt.Sprintf("user-%d", rand.Intn(10000))

	// Start span for database operation
	ctx, span := obsvbrutal.StartSpan(c.Request.Context(), "db.create_user")
	defer span.End()

	// Simulate DB operation
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	span.SetAttributes(
		attribute.String("user.id", userID),
		attribute.String("user.email", req.Email),
	)

	reqLogger.
		Ctx(ctx).
		F("user_id", userID).
		F("duration_ms", time.Since(time.Now()).Milliseconds()).
		Info("User created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"id":         userID,
		"email":      req.Email,
		"name":       req.Name,
		"created_at": time.Now(),
	})
}

// GetUser gets user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	userID := c.Param("id")

	reqLogger.
		F("user_id", userID).
		Debug("Fetching user details")

	// Simulate not found
	if rand.Float32() < 0.1 {
		reqLogger.
			F("user_id", userID).
			Warn("User not found")

		c.JSON(http.StatusNotFound, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":         userID,
		"email":      "user@example.com",
		"name":       "John Doe",
		"created_at": time.Now().Add(-24 * time.Hour),
	})
}

// UpdateUser updates user by ID
func (h *UserHandler) UpdateUser(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	userID := c.Param("id")
	var req struct {
		Name  string `json:"name"`
		Email string `json:"email"`
		Phone string `json:"phone"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid update request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	reqLogger.
		F("user_id", userID).
		F("updates", req).
		Info("Updating user")

	c.JSON(http.StatusOK, gin.H{
		"id":         userID,
		"email":      req.Email,
		"name":       req.Name,
		"updated_at": time.Now(),
	})
}

// DeleteUser deletes user by ID
func (h *UserHandler) DeleteUser(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	userID := c.Param("id")

	reqLogger.
		F("user_id", userID).
		Warn("Deleting user")

	c.JSON(http.StatusNoContent, nil)
}

// OrderHandler handles order-related endpoints
type OrderHandler struct {
	logger obsvbrutal.Logger
}

// NewOrderHandler creates new order handler
func NewOrderHandler(logger obsvbrutal.Logger) *OrderHandler {
	return &OrderHandler{
		logger: logger.Mod("orders"),
	}
}

// CreateOrder creates a new order
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	var req struct {
		UserID  string  `json:"user_id" binding:"required"`
		Items   []Item  `json:"items" binding:"required,min=1"`
		Payment Payment `json:"payment" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid order creation request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
		})
		return
	}

	// Calculate total
	var total float64
	for _, item := range req.Items {
		total += item.Price * float64(item.Quantity)
	}

	orderID := fmt.Sprintf("ord-%d", time.Now().UnixNano())

	// Log order creation with structured data
	reqLogger.
		UID(req.UserID).
		F("order_id", orderID).
		F("total_amount", total).
		F("item_count", len(req.Items)).
		F("payment_method", req.Payment.Method).
		Info("Processing new order")

	// Simulate order processing steps
	ctx := c.Request.Context()

	// Step 1: Validate inventory
	ctx, span1 := obsvbrutal.StartSpan(ctx, "order.validate_inventory")
	time.Sleep(30 * time.Millisecond)
	span1.End()

	// Step 2: Reserve inventory
	ctx, span2 := obsvbrutal.StartSpan(ctx, "order.reserve_inventory")
	time.Sleep(20 * time.Millisecond)
	span2.End()

	// Step 3: Process payment
	ctx, span3 := obsvbrutal.StartSpan(ctx, "order.process_payment")

	// Simulate payment failure
	if rand.Float32() < 0.1 {
		err := fmt.Errorf("payment declined: insufficient funds")
		obsvbrutal.RecordError(span3, err, "Payment processing failed")
		span3.End()

		reqLogger.
			Ctx(ctx).
			Err(err).
			F("order_id", orderID).
			Error("Order creation failed due to payment error")

		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":   "Payment failed",
			"message": err.Error(),
		})
		return
	}

	time.Sleep(50 * time.Millisecond)
	span3.End()

	// Success
	reqLogger.
		Ctx(ctx).
		F("order_id", orderID).
		F("processing_time_ms", time.Since(time.Now()).Milliseconds()).
		Info("Order created successfully")

	c.JSON(http.StatusCreated, gin.H{
		"id":         orderID,
		"user_id":    req.UserID,
		"total":      total,
		"status":     "confirmed",
		"created_at": time.Now(),
	})
}

// GetOrder gets order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	orderID := c.Param("id")

	reqLogger.
		F("order_id", orderID).
		Debug("Fetching order details")

	c.JSON(http.StatusOK, gin.H{
		"id":         orderID,
		"user_id":    "user-123",
		"total":      99.99,
		"status":     "confirmed",
		"created_at": time.Now().Add(-1 * time.Hour),
	})
}

// UpdateOrderStatus updates order status
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	orderID := c.Param("id")
	var req struct {
		Status string `json:"status" binding:"required,oneof=pending confirmed shipped delivered cancelled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid status update")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}

	reqLogger.
		F("order_id", orderID).
		F("new_status", req.Status).
		Info("Updating order status")

	c.JSON(http.StatusOK, gin.H{
		"id":         orderID,
		"status":     req.Status,
		"updated_at": time.Now(),
	})
}

// PaymentHandler handles payment operations
type PaymentHandler struct {
	logger obsvbrutal.Logger
}

// NewPaymentHandler creates new payment handler
func NewPaymentHandler(logger obsvbrutal.Logger) *PaymentHandler {
	return &PaymentHandler{
		logger: logger.Mod("payments"),
	}
}

// ProcessPayment processes a payment
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	var req struct {
		OrderID    string  `json:"order_id" binding:"required"`
		Amount     float64 `json:"amount" binding:"required,gt=0"`
		Method     string  `json:"method" binding:"required,oneof=credit_card debit_card paypal"`
		CardNumber string  `json:"card_number"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid payment request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	paymentID := fmt.Sprintf("pay-%d", time.Now().UnixNano())

	reqLogger.
		F("payment_id", paymentID).
		F("order_id", req.OrderID).
		F("amount", req.Amount).
		F("method", req.Method).
		F("card_number", req.CardNumber). // Will be masked
		Info("Processing payment")

	// Simulate processing
	time.Sleep(200 * time.Millisecond)

	c.JSON(http.StatusCreated, gin.H{
		"id":           paymentID,
		"order_id":     req.OrderID,
		"amount":       req.Amount,
		"status":       "approved",
		"processed_at": time.Now(),
	})
}

// GetPaymentStatus gets payment status
func (h *PaymentHandler) GetPaymentStatus(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	paymentID := c.Param("id")

	reqLogger.
		F("payment_id", paymentID).
		Debug("Fetching payment status")

	c.JSON(http.StatusOK, gin.H{
		"id":           paymentID,
		"status":       "approved",
		"processed_at": time.Now().Add(-5 * time.Minute),
	})
}

// ProcessRefund processes a refund
func (h *PaymentHandler) ProcessRefund(c *gin.Context) {
	reqLogger, _ := obsvbrutal.GetLoggerFromGinContext(c)
	if reqLogger == nil {
		reqLogger = h.logger
	}

	var req struct {
		OrderID string  `json:"order_id" binding:"required"`
		Amount  float64 `json:"amount" binding:"required,gt=0"`
		Reason  string  `json:"reason" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		reqLogger.Err(err).Warn("Invalid refund request")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
		})
		return
	}

	refundID := fmt.Sprintf("ref-%d", time.Now().UnixNano())

	// This will use DEBUG level if configured for payment module
	reqLogger.
		F("refund_id", refundID).
		F("order_id", req.OrderID).
		F("amount", req.Amount).
		F("reason", req.Reason).
		Debug("Processing refund request")

	// Simulate processing
	time.Sleep(100 * time.Millisecond)

	reqLogger.
		F("refund_id", refundID).
		Info("Refund processed successfully")

	c.JSON(http.StatusOK, gin.H{
		"id":           refundID,
		"order_id":     req.OrderID,
		"amount":       req.Amount,
		"status":       "completed",
		"processed_at": time.Now(),
	})
}

// RegisterRoutes registers all HTTP routes
func RegisterRoutes(
	router *gin.Engine,
	userHandler *UserHandler,
	orderHandler *OrderHandler,
	paymentHandler *PaymentHandler,
) {
	api := router.Group("/api")

	// User routes
	users := api.Group("/users")
	{
		users.POST("", userHandler.CreateUser)
		users.GET("/:id", userHandler.GetUser)
	}

	// Order routes
	orders := api.Group("/orders")
	{
		orders.POST("", orderHandler.CreateOrder)
	}

	// Payment routes
	payments := api.Group("/payments")
	{
		payments.POST("/refunds", paymentHandler.ProcessRefund)
	}

	// Test routes
	api.GET("/test/error", func(c *gin.Context) {
		// This will trigger an error log
		panic("test panic")
	})

	api.GET("/test/slow", func(c *gin.Context) {
		// Simulate slow endpoint
		time.Sleep(2 * time.Second)
		c.JSON(http.StatusOK, gin.H{"status": "completed"})
	})
}

// Helper types

type Item struct {
	ProductID string  `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type Payment struct {
	Method     string `json:"method"` // credit_card, debit_card, paypal
	CardNumber string `json:"card_number,omitempty"`
}
