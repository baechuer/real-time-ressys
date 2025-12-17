package consumer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	appErrors "github.com/baechuer/real-time-ressys/services/email-service/app/errors"
	"github.com/baechuer/real-time-ressys/services/email-service/app/idempotency"
	"github.com/baechuer/real-time-ressys/services/email-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/email-service/app/metrics"
	"github.com/baechuer/real-time-ressys/services/email-service/app/retry"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Consumer consumes messages from RabbitMQ
type Consumer struct {
	conn        *amqp.Connection
	ch          *amqp.Channel
	handler     *Handler
	idempotency *idempotency.Checker
	retryConfig *retry.Config
	dlqHandler  *retry.DLQHandler
	workerPool  *WorkerPool
}

// NewConsumer creates a new RabbitMQ consumer
func NewConsumer(
	conn *amqp.Connection,
	ch *amqp.Channel,
	handler *Handler,
	idempotencyChecker *idempotency.Checker,
	retryConfig *retry.Config,
	dlqHandler *retry.DLQHandler,
) *Consumer {
	poolSize := config.GetInt("WORKER_POOL_SIZE", 5)
	workerPool := NewWorkerPool(poolSize)

	return &Consumer{
		conn:        conn,
		ch:          ch,
		handler:     handler,
		idempotency: idempotencyChecker,
		retryConfig: retryConfig,
		dlqHandler:  dlqHandler,
		workerPool:  workerPool,
	}
}

// Start starts consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	// Declare queues
	verificationQueue := config.GetString("RABBITMQ_QUEUE_VERIFICATION", "email.verification.queue")
	resetQueue := config.GetString("RABBITMQ_QUEUE_RESET", "email.password_reset.queue")
	exchangeName := config.GetString("RABBITMQ_EXCHANGE", "auth.events")
	prefetchCount := config.GetInt("PREFETCH_COUNT", 10)

	// Set prefetch count
	err := c.ch.Qos(prefetchCount, 0, false)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Declare and bind verification queue
	_, err = c.ch.QueueDeclare(
		verificationQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    exchangeName,
			"x-dead-letter-routing-key": "email.dlq",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare verification queue: %w", err)
	}

	err = c.ch.QueueBind(
		verificationQueue,
		"email.verification",
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind verification queue: %w", err)
	}

	// Declare and bind reset queue
	_, err = c.ch.QueueDeclare(
		resetQueue,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    exchangeName,
			"x-dead-letter-routing-key": "email.dlq",
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare reset queue: %w", err)
	}

	err = c.ch.QueueBind(
		resetQueue,
		"email.password_reset",
		exchangeName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind reset queue: %w", err)
	}

	// Start consuming from verification queue
	verificationMsgs, err := c.ch.Consume(
		verificationQueue,
		"",    // consumer tag
		false, // auto-ack (we'll ack manually)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer for verification queue: %w", err)
	}

	// Start consuming from reset queue
	resetMsgs, err := c.ch.Consume(
		resetQueue,
		"",    // consumer tag
		false, // auto-ack (we'll ack manually)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer for reset queue: %w", err)
	}

	logger.Logger.Info().Msg("email service consumer started")

	// Process messages in worker pool
	go c.processMessages(ctx, verificationMsgs, verificationQueue)
	go c.processMessages(ctx, resetMsgs, resetQueue)

	// Wait for context cancellation or signal
	<-ctx.Done()
	logger.Logger.Info().Msg("shutting down consumer...")

	// Graceful shutdown: wait for workers to finish
	c.workerPool.Wait()

	return nil
}

// processMessages processes messages from a channel
func (c *Consumer) processMessages(ctx context.Context, msgs <-chan amqp.Delivery, queueName string) {
	for {
		select {
		case <-ctx.Done():
			return
		case delivery, ok := <-msgs:
			if !ok {
				return
			}

			// Record message consumed
			messageType := extractMessageType(delivery.Body)
			metrics.RecordMessageConsumed(queueName, messageType)

			// Submit to worker pool
			c.workerPool.Submit(func() {
				c.handleMessage(ctx, delivery, queueName)
			})
		}
	}
}

// extractMessageType extracts message type from body for metrics
func extractMessageType(body []byte) string {
	// Quick check without full unmarshaling
	bodyStr := string(body)
	if strings.Contains(bodyStr, `"type":"email_verification"`) {
		return "email_verification"
	}
	if strings.Contains(bodyStr, `"type":"password_reset"`) {
		return "password_reset"
	}
	return "unknown"
}

// handleMessage handles a single message
func (c *Consumer) handleMessage(ctx context.Context, delivery amqp.Delivery, queueName string) {
	startTime := time.Now()
	
	// Extract request ID from headers for tracing
	requestID := ""
	if delivery.Headers != nil {
		if rid, ok := delivery.Headers["X-Request-ID"].(string); ok {
			requestID = rid
		}
	}

	// Create context with request ID (using typed key)
	type contextKey string
	const requestIDKey contextKey = "request_id"
	msgCtx := ctx
	if requestID != "" {
		msgCtx = context.WithValue(ctx, requestIDKey, requestID)
	}

	log := logger.WithRequestID(requestID)

	// Validate message body size
	if len(delivery.Body) > 1024*1024 { // 1MB limit
		log.Error().Int("size", len(delivery.Body)).Msg("message body too large")
		metrics.RecordDLQMessage("unknown", "message_too_large")
		delivery.Ack(false) // ACK to remove invalid message
		return
	}

	// Generate message ID for idempotency
	messageID := c.idempotency.GenerateMessageID(delivery)

	// Check idempotency (atomic operation)
	isDuplicate, err := c.idempotency.CheckAndMark(msgCtx, messageID)
	if err != nil {
		log.Error().Err(err).Msg("idempotency check failed - rejecting message to prevent duplicates")
		// FIXED: Reject message if idempotency check fails to prevent data loss
		// NACK and requeue (let RabbitMQ handle retry)
		delivery.Nack(false, true) // requeue=true
		metrics.RecordDLQMessage("unknown", "idempotency_check_failed")
		return
	}
	
	if isDuplicate {
		log.Info().
			Str("message_id", messageID).
			Msg("duplicate message detected, skipping")
		metrics.RecordIdempotencyHit()
		delivery.Ack(false) // ACK duplicate message
		return
	}
	
	metrics.RecordIdempotencyMiss()

	// Extract message type for metrics
	messageType := extractMessageType(delivery.Body)

	// Process message with retry
	var processingErr error
	retryAttempts := 0
	err = retry.Retry(msgCtx, c.retryConfig, func() error {
		if retryAttempts > 0 {
			metrics.RecordRetryAttempt(messageType)
		}
		retryAttempts++
		processingErr = c.handler.ProcessMessage(msgCtx, delivery.Body)
		return processingErr
	})

	// Record processing duration
	duration := time.Since(startTime)
	metrics.RecordMessageProcessing(messageType, duration)

	if err != nil {
		log.Error().Err(err).Msg("failed to process message after retries")

		// Determine error type for metrics
		errorType := "unknown"
		if appErr, ok := err.(*appErrors.AppError); ok {
			errorType = string(appErr.Code)
		}

		// Check if error is retryable
		if retry.IsRetryable(err) {
			// Send to DLQ
			dlqErr := c.dlqHandler.PublishToDLQ(msgCtx, delivery, err.Error())
			if dlqErr != nil {
				log.Error().Err(dlqErr).Msg("failed to publish to DLQ - message will be lost")
				// FIXED: Don't ACK if DLQ publish fails - NACK to requeue
				delivery.Nack(false, true) // requeue to try again
				metrics.RecordDLQMessage(messageType, "dlq_publish_failed")
				return
			}
			metrics.RecordDLQMessage(messageType, errorType)
			delivery.Ack(false) // ACK to remove from queue (it's in DLQ now)
		} else {
			// Permanent failure - ACK and log
			log.Error().Err(err).Msg("permanent failure, acknowledging message")
			metrics.RecordDLQMessage(messageType, "permanent_failure")
			delivery.Ack(false)
		}
		return
	}

	// Success - ACK message
	delivery.Ack(false)
	log.Info().Msg("message processed successfully")
}

// Close closes the consumer
func (c *Consumer) Close() error {
	c.workerPool.Stop()
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

