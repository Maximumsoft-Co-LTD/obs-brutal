package example

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"obs-brutal/internal/app/config"
	"obs-brutal/obsvbrutal"

	"github.com/streadway/amqp"
	"go.uber.org/fx"
)

// AMQPExample demonstrates AMQP integration
type AMQPExample struct {
	logger   obsvbrutal.Logger
	provider *obsvbrutal.OTelProvider
	cfg      *config.ObsConfig
}

// NewAMQPExample creates AMQP example
func NewAMQPExample(
	logger obsvbrutal.Logger,
	provider *obsvbrutal.OTelProvider,
	cfg *config.ObsConfig,
) *AMQPExample {
	return &AMQPExample{
		logger:   logger.Mod("amqp"),
		provider: provider,
		cfg:      cfg,
	}
}

// StartConsumer starts example AMQP consumer
func (e *AMQPExample) StartConsumer(ctx context.Context) error {
	if e.cfg == nil || !e.cfg.AMQP.ENABLED {
		e.logger.Info("AMQP is disabled; skipping consumer startup")
		return nil
	}

	// Use configured AMQP URL
	amqpURL := e.cfg.AMQP.URL

	// Connect to RabbitMQ
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		e.logger.Err(err).Warn("Failed to connect to RabbitMQ")
		return nil // Don't fail the app if RabbitMQ is not available
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		e.logger.Err(err).Error("Failed to open channel")
		return err
	}
	defer ch.Close()

	// Declare exchange
	err = ch.ExchangeDeclare(
		"events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		e.logger.Err(err).Error("Failed to declare exchange")
		return err
	}

	// Declare queues
	queues := []struct {
		name       string
		routingKey string
		handler    obsvbrutal.AMQPHandler
	}{
		{
			name:       "orders",
			routingKey: "order.*",
			handler:    e.handleOrderMessage,
		},
		{
			name:       "payments",
			routingKey: "payment.*",
			handler:    e.handlePaymentMessage,
		},
		{
			name:       "notifications",
			routingKey: "notification.*",
			handler:    e.handleNotificationMessage,
		},
	}

	// Create consumers for each queue
	for _, q := range queues {
		queue, err := ch.QueueDeclare(
			q.name,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			e.logger.Err(err).F("queue", q.name).Error("Failed to declare queue")
			continue
		}

		err = ch.QueueBind(
			queue.Name,
			q.routingKey,
			"events",
			false,
			nil,
		)
		if err != nil {
			e.logger.Err(err).F("queue", q.name).Error("Failed to bind queue")
			continue
		}

		// Create consumer
		consumer := obsvbrutal.NewAMQPConsumer(ch, e.logger, e.provider)

		// Start consuming in goroutine
		go func(queueName string, handler obsvbrutal.AMQPHandler) {
			e.logger.F("queue", queueName).Info("Starting AMQP consumer")

			// Wrap with dead letter handling
			wrappedHandler := obsvbrutal.DeadLetterHandler(
				handler,
				obsvbrutal.NewAMQPPublisher(ch, e.logger, e.provider),
				"dlx",
				"dead_letter",
				3, // max retries
			)

			if err := consumer.Consume(ctx, queueName, wrappedHandler); err != nil {
				e.logger.Err(err).F("queue", queueName).Error("Consumer error")
			}
		}(q.name, q.handler)
	}

	// Also start a batch consumer example
	go e.startBatchConsumer(ctx, ch)

	// Also start a publisher example
	go e.startPublisher(ctx, ch)

	return nil
}

// handleOrderMessage handles order messages
func (e *AMQPExample) handleOrderMessage(ctx context.Context, msg amqp.Delivery) error {
	logger := e.getMessageLogger(ctx)

	var order struct {
		ID     string  `json:"id"`
		UserID string  `json:"user_id"`
		Total  float64 `json:"total"`
		Status string  `json:"status"`
	}

	if err := json.Unmarshal(msg.Body, &order); err != nil {
		logger.Err(err).Error("Failed to unmarshal order message")
		return err
	}

	logger.
		F("order_id", order.ID).
		F("user_id", order.UserID).
		F("total", order.Total).
		F("status", order.Status).
		Info("Processing order event")

	// Simulate processing
	time.Sleep(50 * time.Millisecond)

	// Simulate occasional error
	if order.Total > 10000 {
		return fmt.Errorf("order amount too high for automatic processing")
	}

	logger.F("order_id", order.ID).Info("Order event processed successfully")
	return nil
}

// handlePaymentMessage handles payment messages
func (e *AMQPExample) handlePaymentMessage(ctx context.Context, msg amqp.Delivery) error {
	logger := e.getMessageLogger(ctx)

	var payment struct {
		ID      string  `json:"id"`
		OrderID string  `json:"order_id"`
		Amount  float64 `json:"amount"`
		Method  string  `json:"method"`
		Status  string  `json:"status"`
	}

	if err := json.Unmarshal(msg.Body, &payment); err != nil {
		logger.Err(err).Error("Failed to unmarshal payment message")
		return err
	}

	logger.
		F("payment_id", payment.ID).
		F("order_id", payment.OrderID).
		F("amount", payment.Amount).
		F("method", payment.Method).
		Info("Processing payment event")

	// Simulate processing
	time.Sleep(100 * time.Millisecond)

	logger.F("payment_id", payment.ID).Info("Payment event processed successfully")
	return nil
}

// handleNotificationMessage handles notification messages
func (e *AMQPExample) handleNotificationMessage(ctx context.Context, msg amqp.Delivery) error {
	logger := e.getMessageLogger(ctx)

	var notification struct {
		Type      string                 `json:"type"`
		Recipient string                 `json:"recipient"`
		Template  string                 `json:"template"`
		Data      map[string]interface{} `json:"data"`
	}

	if err := json.Unmarshal(msg.Body, &notification); err != nil {
		logger.Err(err).Error("Failed to unmarshal notification message")
		return err
	}

	logger.
		F("type", notification.Type).
		F("recipient", notification.Recipient).
		F("template", notification.Template).
		Debug("Sending notification")

	// Simulate sending
	time.Sleep(30 * time.Millisecond)

	return nil
}

// startBatchConsumer demonstrates batch consuming
func (e *AMQPExample) startBatchConsumer(ctx context.Context, ch *amqp.Channel) {
	// Create batch queue
	queue, err := ch.QueueDeclare(
		"batch_events",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		e.logger.Err(err).Error("Failed to declare batch queue")
		return
	}

	// Create batch consumer
	batchConsumer := obsvbrutal.NewAMQPBatchConsumer(
		ch,
		e.logger,
		e.provider,
		10,            // batch size
		2*time.Second, // batch timeout
	)

	// Batch handler
	batchHandler := func(ctx context.Context, messages []amqp.Delivery) error {
		logger := e.logger.F("batch_size", len(messages))
		logger.Info("Processing message batch")

		// Process all messages
		for _, msg := range messages {
			// Process each message
			logger.F("message_id", msg.MessageId).Debug("Processing batched message")
		}

		logger.Info("Batch processed successfully")
		return nil
	}

	e.logger.Info("Starting batch consumer")
	if err := batchConsumer.ConsumeBatch(ctx, queue.Name, batchHandler); err != nil {
		e.logger.Err(err).Error("Batch consumer error")
	}
}

// startPublisher demonstrates publishing with logging
func (e *AMQPExample) startPublisher(ctx context.Context, ch *amqp.Channel) {
	publisher := obsvbrutal.NewAMQPPublisher(ch, e.logger, e.provider)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Publish test events
			events := []struct {
				exchange string
				key      string
				message  interface{}
			}{
				{
					exchange: "events",
					key:      "order.created",
					message: map[string]interface{}{
						"id":      fmt.Sprintf("ord-%d", time.Now().Unix()),
						"user_id": "user-123",
						"total":   99.99,
						"status":  "pending",
					},
				},
				{
					exchange: "events",
					key:      "payment.processed",
					message: map[string]interface{}{
						"id":       fmt.Sprintf("pay-%d", time.Now().Unix()),
						"order_id": fmt.Sprintf("ord-%d", time.Now().Unix()-1),
						"amount":   99.99,
						"method":   "credit_card",
						"status":   "completed",
					},
				},
			}

			for _, event := range events {
				if err := publisher.Publish(ctx, event.exchange, event.key, event.message); err != nil {
					e.logger.Err(err).Error("Failed to publish event")
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// getMessageLogger gets logger from context or returns default
func (e *AMQPExample) getMessageLogger(ctx context.Context) obsvbrutal.Logger {
	if logger, ok := obsvbrutal.GetLoggerFromContext(ctx); ok {
		return logger
	}
	return e.logger
}

// AMQPParams for FX
type AMQPParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Example   *AMQPExample
}

// StartBackgroundWorkers starts all background workers
func StartBackgroundWorkers(params AMQPParams) {
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Start AMQP consumer
			go params.Example.StartConsumer(context.Background())
			return nil
		},
	})
}
