package shared

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/streadway/amqp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	pin "github.com/Maximumsoft-Co-LTD/obs-brutal/internal/core/port/inbound"
)

// AMQPConsumerMiddleware provides logging middleware for AMQP consumers
type AMQPConsumerMiddleware struct {
	logger   pin.Logger
	provider *OTelProvider
}

// NewAMQPConsumerMiddleware creates new AMQP consumer middleware
func NewAMQPConsumerMiddleware(logger pin.Logger, provider *OTelProvider) *AMQPConsumerMiddleware {
	return &AMQPConsumerMiddleware{
		logger:   logger,
		provider: provider,
	}
}

// Wrap wraps an AMQP handler with logging and tracing
func (m *AMQPConsumerMiddleware) Wrap(handler AMQPHandler) AMQPHandler {
	return func(ctx context.Context, msg amqp.Delivery) error {
		start := time.Now()

		// Extract trace context from headers
		if m.provider != nil && msg.Headers != nil {
			ctx = ExtractAMQPHeaders(ctx, msg.Headers)
		}

		// Start span if provider available
		var span trace.Span
		if m.provider != nil {
			ctx, span = m.provider.tracer.Start(ctx,
				fmt.Sprintf("amqp.consume %s", msg.RoutingKey),
				trace.WithAttributes(
					attribute.String("messaging.system", "rabbitmq"),
					attribute.String("messaging.destination", msg.Exchange),
					attribute.String("messaging.routing_key", msg.RoutingKey),
					attribute.String("messaging.message_id", msg.MessageId),
					attribute.String("messaging.correlation_id", msg.CorrelationId),
					attribute.Int64("messaging.message_payload_size_bytes", int64(len(msg.Body))),
				),
			)
			defer span.End()
		}

		// Extract message metadata
		messageID := msg.MessageId
		if messageID == "" {
			messageID = generateRequestID()
		}

		// Create logger with context
		msgLogger := m.logger.
			Ctx(ctx).
			F("message_id", messageID).
			F("exchange", msg.Exchange).
			F("routing_key", msg.RoutingKey).
			F("correlation_id", msg.CorrelationId).
			F("content_type", msg.ContentType).
			F("delivery_tag", msg.DeliveryTag).
			F("redelivered", msg.Redelivered)

		// Extract custom headers
		if msg.Headers != nil {
			if userID, ok := msg.Headers["user_id"].(string); ok {
				msgLogger = msgLogger.UID(userID)
			}
			if tenantID, ok := msg.Headers["tenant_id"].(string); ok {
				msgLogger = msgLogger.Tenant(tenantID)
			}
			if requestID, ok := msg.Headers["request_id"].(string); ok {
				msgLogger = msgLogger.RID(requestID)
			}
		}

		// Log message consumption start
		msgLogger.Info("AMQP message consumption started")

		// Store logger in context
		ctx = context.WithValue(ctx, "logger", msgLogger)

		// Process message
		err := handler(ctx, msg)

		// Calculate duration
		duration := time.Since(start)

		// Log result
		finalLogger := msgLogger.F("duration_ms", duration.Milliseconds())

		if err != nil {
			finalLogger.Err(err).Error("AMQP message consumption failed")
			if span != nil {
				RecordError(span, err, "Message processing failed")
			}
		} else {
			finalLogger.Info("AMQP message consumption completed")
		}

		return err
	}
}

// AMQPHandler represents an AMQP message handler
type AMQPHandler func(ctx context.Context, msg amqp.Delivery) error

// AMQPPublisher provides logging for AMQP publishing
type AMQPPublisher struct {
	channel  *amqp.Channel
	logger   pin.Logger
	provider *OTelProvider
}

// NewAMQPPublisher creates new AMQP publisher with logging
func NewAMQPPublisher(channel *amqp.Channel, logger pin.Logger, provider *OTelProvider) *AMQPPublisher {
	return &AMQPPublisher{
		channel:  channel,
		logger:   logger,
		provider: provider,
	}
}

// Publish publishes a message with logging and tracing
func (p *AMQPPublisher) Publish(ctx context.Context, exchange, key string, message interface{}) error {
	// Start span if provider available
	var span trace.Span
	if p.provider != nil {
		ctx, span = p.provider.tracer.Start(ctx,
			fmt.Sprintf("amqp.publish %s", key),
			trace.WithAttributes(
				attribute.String("messaging.system", "rabbitmq"),
				attribute.String("messaging.destination", exchange),
				attribute.String("messaging.routing_key", key),
			),
		)
		defer span.End()
	}

	// Generate message ID
	messageID := generateRequestID()

	// Marshal message
	body, err := json.Marshal(message)
	if err != nil {
		p.logger.Ctx(ctx).Err(err).Error("Failed to marshal AMQP message")
		if span != nil {
			RecordError(span, err, "Message marshaling failed")
		}
		return err
	}

	// Prepare headers
	headers := amqp.Table{}

	// Inject trace context
	if p.provider != nil {
		InjectAMQPHeaders(ctx, headers)
	}

	// Add custom headers from context
	if userID := ctx.Value("user_id"); userID != nil {
		headers["user_id"] = userID
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		headers["tenant_id"] = tenantID
	}
	if requestID := ctx.Value("request_id"); requestID != nil {
		headers["request_id"] = requestID
	}

	// Create logger
	pubLogger := p.logger.
		Ctx(ctx).
		F("message_id", messageID).
		F("exchange", exchange).
		F("routing_key", key).
		F("size", len(body))

	// Log publish attempt
	pubLogger.Info("Publishing AMQP message")

	// Publish message
	err = p.channel.Publish(
		exchange,
		key,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			Headers:      headers,
			ContentType:  "application/json",
			MessageId:    messageID,
			Timestamp:    time.Now(),
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)

	if err != nil {
		pubLogger.Err(err).Error("Failed to publish AMQP message")
		if span != nil {
			RecordError(span, err, "Message publishing failed")
		}
		return err
	}

	pubLogger.Info("AMQP message published successfully")

	if span != nil {
		span.SetAttributes(
			attribute.String("messaging.message_id", messageID),
			attribute.Int64("messaging.message_payload_size_bytes", int64(len(body))),
		)
	}

	return nil
}

// AMQPConsumer provides structured AMQP consumer with logging
type AMQPConsumer struct {
	channel    *amqp.Channel
	logger     pin.Logger
	provider   *OTelProvider
	middleware *AMQPConsumerMiddleware
}

// NewAMQPConsumer creates new AMQP consumer
func NewAMQPConsumer(channel *amqp.Channel, logger pin.Logger, provider *OTelProvider) *AMQPConsumer {
	return &AMQPConsumer{
		channel:    channel,
		logger:     logger,
		provider:   provider,
		middleware: NewAMQPConsumerMiddleware(logger, provider),
	}
}

// Consume starts consuming messages from a queue
func (c *AMQPConsumer) Consume(ctx context.Context, queue string, handler AMQPHandler) error {
	// Create consumer tag
	consumerTag := fmt.Sprintf("logbrutal-%s-%d", queue, time.Now().UnixNano())

	// Start consuming
	msgs, err := c.channel.Consume(
		queue,
		consumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		c.logger.Err(err).Error("Failed to start AMQP consumer")
		return err
	}

	c.logger.
		F("queue", queue).
		F("consumer_tag", consumerTag).
		Info("AMQP consumer started")

	// Wrap handler with middleware
	wrappedHandler := c.middleware.Wrap(handler)

	// Process messages
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				c.logger.Warn("AMQP channel closed")
				return nil
			}

			// Process message in goroutine for concurrent handling
			go func(delivery amqp.Delivery) {
				// Create message context
				msgCtx := context.Background()

				// Process with wrapped handler
				if err := wrappedHandler(msgCtx, delivery); err != nil {
					// Negative acknowledge on error
					delivery.Nack(false, true)
				} else {
					// Acknowledge on success
					delivery.Ack(false)
				}
			}(msg)

		case <-ctx.Done():
			c.logger.Info("AMQP consumer stopping due to context cancellation")

			// Cancel consumer
			if err := c.channel.Cancel(consumerTag, false); err != nil {
				c.logger.Err(err).Error("Failed to cancel AMQP consumer")
			}

			return ctx.Err()
		}
	}
}

// AMQPBatchConsumer consumes messages in batches for better performance
type AMQPBatchConsumer struct {
	*AMQPConsumer
	batchSize    int
	batchTimeout time.Duration
}

// NewAMQPBatchConsumer creates new batch consumer
func NewAMQPBatchConsumer(channel *amqp.Channel, logger pin.Logger, provider *OTelProvider, batchSize int, batchTimeout time.Duration) *AMQPBatchConsumer {
	return &AMQPBatchConsumer{
		AMQPConsumer: NewAMQPConsumer(channel, logger, provider),
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
	}
}

// ConsumeBatch consumes messages in batches
func (c *AMQPBatchConsumer) ConsumeBatch(ctx context.Context, queue string, handler AMQPBatchHandler) error {
	consumerTag := fmt.Sprintf("logbrutal-batch-%s-%d", queue, time.Now().UnixNano())

	msgs, err := c.channel.Consume(
		queue,
		consumerTag,
		false, // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		c.logger.Err(err).Error("Failed to start AMQP batch consumer")
		return err
	}

	c.logger.
		F("queue", queue).
		F("consumer_tag", consumerTag).
		F("batch_size", c.batchSize).
		Info("AMQP batch consumer started")

	// Process messages in batches
	batch := make([]amqp.Delivery, 0, c.batchSize)
	timer := time.NewTimer(c.batchTimeout)

	processBatch := func() {
		if len(batch) == 0 {
			return
		}

		batchLogger := c.logger.F("batch_size", len(batch))
		batchLogger.Info("Processing AMQP message batch")

		// Process batch
		if err := handler(ctx, batch); err != nil {
			batchLogger.Err(err).Error("Failed to process AMQP batch")
			// Nack all messages in batch
			for _, msg := range batch {
				msg.Nack(false, true)
			}
		} else {
			// Ack all messages in batch
			for _, msg := range batch {
				msg.Ack(false)
			}
			batchLogger.Info("AMQP batch processed successfully")
		}

		// Clear batch
		batch = batch[:0]
	}

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				processBatch()
				c.logger.Warn("AMQP channel closed")
				return nil
			}

			batch = append(batch, msg)

			if len(batch) >= c.batchSize {
				processBatch()
				timer.Reset(c.batchTimeout)
			}

		case <-timer.C:
			processBatch()
			timer.Reset(c.batchTimeout)

		case <-ctx.Done():
			processBatch()
			c.logger.Info("AMQP batch consumer stopping due to context cancellation")

			if err := c.channel.Cancel(consumerTag, false); err != nil {
				c.logger.Err(err).Error("Failed to cancel AMQP batch consumer")
			}

			return ctx.Err()
		}
	}
}

// AMQPBatchHandler handles a batch of messages
type AMQPBatchHandler func(ctx context.Context, messages []amqp.Delivery) error

// DeadLetterHandler wraps handler with dead letter queue support
func DeadLetterHandler(handler AMQPHandler, publisher *AMQPPublisher, dlExchange, dlRoutingKey string, maxRetries int) AMQPHandler {
	return func(ctx context.Context, msg amqp.Delivery) error {
		// Get retry count from headers
		retryCount := 0
		if msg.Headers != nil {
			if count, ok := msg.Headers["x-retry-count"].(int32); ok {
				retryCount = int(count)
			}
		}

		// Process message
		err := handler(ctx, msg)
		if err == nil {
			return nil
		}

		// Check if we should retry
		if retryCount >= maxRetries {
			// Send to dead letter queue
			dlMessage := map[string]interface{}{
				"original_exchange":    msg.Exchange,
				"original_routing_key": msg.RoutingKey,
				"error":                err.Error(),
				"retry_count":          retryCount,
				"timestamp":            time.Now(),
				"body":                 string(msg.Body),
			}

			if pubErr := publisher.Publish(ctx, dlExchange, dlRoutingKey, dlMessage); pubErr != nil {
				publisher.logger.Err(pubErr).Error("Failed to publish to dead letter queue")
			}

			publisher.logger.
				F("retry_count", retryCount).
				F("max_retries", maxRetries).
				Err(err).
				Error("Message sent to dead letter queue after max retries")

			return nil // Acknowledge to remove from queue
		}

		// Republish with incremented retry count
		headers := msg.Headers
		if headers == nil {
			headers = amqp.Table{}
		}
		headers["x-retry-count"] = int32(retryCount + 1)
		headers["x-last-error"] = err.Error()

		republishErr := publisher.channel.Publish(
			msg.Exchange,
			msg.RoutingKey,
			false,
			false,
			amqp.Publishing{
				Headers:       headers,
				ContentType:   msg.ContentType,
				MessageId:     msg.MessageId,
				CorrelationId: msg.CorrelationId,
				Timestamp:     msg.Timestamp,
				Body:          msg.Body,
				DeliveryMode:  msg.DeliveryMode,
			},
		)

		if republishErr != nil {
			publisher.logger.Err(republishErr).Error("Failed to republish message for retry")
			return republishErr
		}

		publisher.logger.
			F("retry_count", retryCount+1).
			Err(err).
			Warn("Message requeued for retry")

		return nil // Acknowledge original message
	}
}

func generateRequestID() string {
	return fmt.Sprintf("req-%d-%d", time.Now().UnixNano(), os.Getpid())
}
