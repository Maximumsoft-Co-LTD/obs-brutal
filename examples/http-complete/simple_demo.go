package example

import (
	"fmt"
	"math/rand"
	"net/http"
	"obs-brutal/pkg/obsvbrutal"
	"time"

	"github.com/gin-gonic/gin"
)

// SimpleDemoHandler demonstrates the simple API
type SimpleDemoHandler struct {
	logger obsvbrutal.Logger
}

// NewSimpleDemoHandler creates new demo handler
func NewSimpleDemoHandler(logger obsvbrutal.Logger) *SimpleDemoHandler {
	return &SimpleDemoHandler{
		logger: logger.Mod("simple_demo"),
	}
}

// SimpleDemo shows basic simple API usage
func (h *SimpleDemoHandler) SimpleDemo(c *gin.Context) {
	logFrmGin := obsvbrutal.GetLogFrmGin(c, "SimpleDemo")
	defer logFrmGin.Close()

	logFrmGin.Prt("Simple API demonstration")

	// Simple response
	logFrmGin.R(http.StatusOK,
		obsvbrutal.OptsResponse().
			Response(gin.H{
				"message": "Simple API demo",
				"time":    time.Now().Format(time.RFC3339),
			}),
	)
}

// FluentDemo demonstrates fluent interface
func (h *SimpleDemoHandler) FluentDemo(c *gin.Context) {
	logFrmGin := obsvbrutal.GetLogFrmGin(c, "FluentDemo")
	defer logFrmGin.Close()

	// Fluent field addition
	logFrmGin.F("user_id", "123").
		F("action", "demo").
		F("ip", c.ClientIP()).
		Prt("Fluent interface demonstration")

	// Response with fluent options
	var opts = obsvbrutal.OptsResponse()
	logFrmGin.R(http.StatusOK,
		opts.Msg("Fluent API demonstrated"),
		opts.Response(gin.H{"status": "success"}),
	)
}

// TraceDemo demonstrates tracing functionality
func (h *SimpleDemoHandler) TraceDemo(c *gin.Context) {
	logFrmGin := obsvbrutal.GetLogFrmGin(c, "TraceDemo")
	defer logFrmGin.Close()

	// Start parent trace
	trc := logFrmGin.FlatPr("trace.demo")
	defer trc.End()

	// Add trace attributes
	trc.Add(
		trc.Str("demo", "trace"),
		trc.Bool("enabled", true),
		trc.Num("version", 1.0),
	)

	// Child traces
	childTrc := trc.FlatPr("trace.child")
	trc.Add(childTrc.Detail("Processing child trace"))
	time.Sleep(50 * time.Millisecond)
	childTrc.End()

	// Response
	logFrmGin.R(http.StatusOK,
		obsvbrutal.OptsResponse().
			Msg("Trace demo completed"),
		obsvbrutal.OptsResponse().
			Response(gin.H{
				"trace_id": logFrmGin.GetTraceID(),
				"span_id":  logFrmGin.GetSpanID(),
			}),
	)
}

// CreateUserSimple demonstrates simple API usage
func (h *SimpleDemoHandler) CreateUserSimple(c *gin.Context) {
	// Define request struct with log tags
	type Body struct {
		Email    string `json:"email" binding:"required,email"`
		Name     string `log:"name" json:"name" binding:"required"`
		Password string `log:"password,sensitive=true" json:"password" binding:"required,min=8"`
		Phone    string `log:"phone,mask=00000000xxx" json:"phone"`
		IDCard   string `log:"id_card,sensitive=true" json:"id_card"` // Thai ID card
	}

	// Get logger from Gin context
	logFrmGin := obsvbrutal.GetLogFrmGin(c, "CreateUser")
	defer logFrmGin.Close()

	var opts = obsvbrutal.OptsResponse()

	// Bind request
	var req Body
	if err := c.ShouldBindJSON(&req); err != nil {
		logFrmGin.R(http.StatusBadRequest,
			opts.Detail(err.Error()),
			opts.Msg("Invalid user creation request"),
		).Err(err)
		return
	}

	// Log with struct fields (sensitive data will be masked)
	fields := obsvbrutal.ExtractFields(req)
	for k, v := range fields {
		logFrmGin.F(k, v)
	}
	logFrmGin.Prtf("Creating new user: %s", req.Email)

	// Generate user ID
	userID := fmt.Sprintf("user-%d", rand.Intn(10000))

	// Create parent trace for DB operation
	dbTrace := logFrmGin.FlatPr("db.create_user")
	defer dbTrace.End()

	// Simulate DB operation
	time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)

	// Add trace attributes
	dbTrace.Add(
		dbTrace.Str("user.id", userID),
		dbTrace.Str("user.email", req.Email),
		dbTrace.Bool("user.is_active", true),
		dbTrace.Num("user.age", 25),
	)
	// Handle body separately
	// dbTrace.Body("user.request", req)

	// Process with sub-operations
	// TODO: Fix type - processUserCreation expects *Parent, not Tracer interface
	// if err := h.processUserCreation(dbTrace, userID); err != nil {
	// 	rb := logFrmGin.R(http.StatusInternalServerError)
	// 	rb.Err(err)
	// 	return
	// }

	// Success response
	c.JSON(http.StatusCreated, gin.H{
		"id":         userID,
		"email":      req.Email,
		"name":       req.Name,
		"created_at": time.Now(),
	})
}

// processUserCreation demonstrates nested traces
func (h *SimpleDemoHandler) processUserCreation(parentTrace *obsvbrutal.Parent, userID string) error {
	// Validate user data
	valTrc := parentTrace.Parent("validate_user_data")
	time.Sleep(30 * time.Millisecond)
	valTrc.End()

	// Check duplicate
	dupTrc := parentTrace.Parent("check_duplicate")
	time.Sleep(20 * time.Millisecond)
	dupTrc.End()

	// Create user record
	createTrace := parentTrace.Parent("create_user_record")
	time.Sleep(50 * time.Millisecond)

	// Simulate occasional error
	if rand.Float32() < 0.1 {
		return createTrace.Errf("Failed to create user: %w", fmt.Errorf("database connection error"))
	}

	createTrace.Add(createTrace.Str("user.id", userID))
	createTrace.End()

	// Send welcome email
	emailTrace := parentTrace.Parent("send_welcome_email")
	time.Sleep(100 * time.Millisecond)
	emailTrace.Add(
		emailTrace.Str("email.template", "welcome"),
		emailTrace.Bool("email.sent", true),
	)
	emailTrace.End()

	return nil
}

// ProcessPaymentSimple demonstrates payment processing with simple API
func (h *SimpleDemoHandler) ProcessPaymentSimple(c *gin.Context) {
	type PaymentRequest struct {
		OrderID      string  `json:"order_id" binding:"required"`
		Amount       float64 `json:"amount" binding:"required,gt=0"`
		CardNumber   string  `log:"card_number,mask=****xxxx" json:"card_number"`
		CVV          string  `log:"-" json:"cvv"` // Don't log CVV
		CustomerName string  `log:"customer_name" json:"customer_name"`
	}

	logFrmGin := obsvbrutal.GetLogFrmGin(c, "ProcessPayment")
	defer logFrmGin.Close()

	var req PaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logFrmGin.R(http.StatusBadRequest,
			obsvbrutal.OptsResponse().Detail(err.Error()),
		).Err(err)
		return
	}

	// Auto-log struct fields
	fields := obsvbrutal.ExtractFields(req)
	for k, v := range fields {
		logFrmGin.F(k, v)
	}

	// Create payment trace with tag
	paymentTrace := logFrmGin.FlatPr("payment_processing")
	paymentTrace.Add(paymentTrace.Str("order", req.OrderID))
	defer paymentTrace.End()

	// Validate payment
	validateTrace := paymentTrace.FlatPr("validate_payment")
	if req.Amount > 10000 {
		err := validateTrace.Errf("Amount too high: %.2f", req.Amount)
		validateTrace.End()
		rb := logFrmGin.R(http.StatusBadRequest,
			obsvbrutal.OptsResponse().Msg("Payment validation failed"),
		)
		rb.Err(err)
		return
	}
	validateTrace.End()

	// Process payment
	processTrace := paymentTrace.FlatPr("process_payment")
	time.Sleep(200 * time.Millisecond)

	// Simulate payment result
	if rand.Float32() < 0.2 {
		err := processTrace.Errf("Payment declined: insufficient funds")
		rb := logFrmGin.R(http.StatusPaymentRequired)
		rb.Err(err)
		return
	}

	transactionID := fmt.Sprintf("txn-%d", time.Now().Unix())
	processTrace.Add(
		processTrace.Str("transaction.id", transactionID),
		processTrace.Num("transaction.amount", req.Amount),
		processTrace.Code(http.StatusOK),
	)
	processTrace.End()

	// Success
	c.JSON(http.StatusOK, gin.H{
		"transaction_id": transactionID,
		"order_id":       req.OrderID,
		"amount":         req.Amount,
		"status":         "completed",
		"processed_at":   time.Now(),
	})
}

// RegisterSimpleRoutes registers simple demo routes
func RegisterSimpleRoutes(router *gin.Engine, handler *SimpleDemoHandler) {
	api := router.Group("/api/v2")

	// User routes
	users := api.Group("/users")
	{
		users.POST("", handler.CreateUserSimple)
	}

	// Payment routes
	payments := api.Group("/payments")
	{
		payments.POST("", handler.ProcessPaymentSimple)
	}
}
