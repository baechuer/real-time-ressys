package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/baechuer/real-time-ressys/services/email-service/app/config"
	"github.com/baechuer/real-time-ressys/services/email-service/app/consumer"
	"github.com/baechuer/real-time-ressys/services/email-service/app/email"
	"github.com/baechuer/real-time-ressys/services/email-service/app/health"
	"github.com/baechuer/real-time-ressys/services/email-service/app/idempotency"
	"github.com/baechuer/real-time-ressys/services/email-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/email-service/app/metrics"
	"github.com/baechuer/real-time-ressys/services/email-service/app/ratelimit"
	"github.com/baechuer/real-time-ressys/services/email-service/app/retry"
)

func main() {
	// Initialize logger
	logger.Init()
	log := logger.Logger

	// Load configuration
	if err := config.Load(); err != nil {
		log.Warn().Err(err).Msg("failed to load .env file, using environment variables")
	}

	// Connect to RabbitMQ
	conn, ch, err := config.NewRabbitMQConnection()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
	defer conn.Close()
	defer ch.Close()

	// Connect to Redis (for idempotency)
	redisClient, err := config.NewRedisClient()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	// Initialize email sender
	emailConfig := config.LoadEmailConfig()
	emailSender, err := email.NewSender(emailConfig)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize email sender")
	}

	// Initialize idempotency
	idempotencyStore := idempotency.NewStore(redisClient)
	idempotencyChecker := idempotency.NewChecker(idempotencyStore)

	// Initialize retry config
	retryConfig := retry.LoadConfig()

	// Initialize DLQ handler
	dlqName := config.GetString("RABBITMQ_QUEUE_DLQ", "email.dlq")
	dlqHandler, err := retry.NewDLQHandler(ch, dlqName)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize DLQ handler")
	}

	// Initialize rate limiter
	rateLimiter := ratelimit.NewRateLimiter(redisClient)

	// Initialize handler
	msgHandler := consumer.NewHandler(emailSender, rateLimiter)

	// Initialize consumer
	msgConsumer := consumer.NewConsumer(
		conn,
		ch,
		msgHandler,
		idempotencyChecker,
		retryConfig,
		dlqHandler,
	)

	// Initialize health check handler
	healthHandler := health.NewHandler(conn, ch, redisClient, emailSender)

	// Setup HTTP server for health checks and metrics
	healthPort := config.GetString("HEALTH_CHECK_PORT", "8081")
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler.HealthCheck)
	mux.HandleFunc("/health/rabbitmq", healthHandler.HealthCheckRabbitMQ)
	mux.HandleFunc("/health/redis", healthHandler.HealthCheckRedis)
	mux.HandleFunc("/health/email", healthHandler.HealthCheckEmail)

	// Prometheus metrics endpoint
	metricsHandler := metrics.MetricsHandler()
	mux.Handle("/metrics", metricsHandler)

	healthServer := &http.Server{
		Addr:    ":" + healthPort,
		Handler: mux,
	}

	// Start health check server
	go func() {
		log.Info().Str("port", healthPort).Msg("starting health check server")
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error().Err(err).Msg("health check server failed")
		}
	}()

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Info().Str("signal", sig.String()).Msg("received shutdown signal")
		cancel()
	}()

	// Start consumer
	log.Info().Msg("starting email service...")
	go func() {
		if err := msgConsumer.Start(ctx); err != nil {
			log.Error().Err(err).Msg("consumer failed")
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	log.Info().Msg("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown health check server
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("error shutting down health check server")
	}

	// Close consumer
	msgConsumer.Close()

	select {
	case <-shutdownCtx.Done():
		log.Warn().Msg("shutdown timeout exceeded")
	default:
		log.Info().Msg("shutdown complete")
	}
}
