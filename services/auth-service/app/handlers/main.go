package main

import (
	"fmt"
	"os"

	cfgPkg "github.com/baechuer/real-time-ressys/services/auth-service/app/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/logger"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/services"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/store"
)

func main() {
	// Initialize structured logging
	logger.Init()

	// Load .env file (if it exists)
	cfgPkg.Load()

	if err := validateRequiredEnv(); err != nil {
		logger.Logger.Fatal().Err(err).Msg("required environment variables missing")
	}

	// Build connection string from individual components
	dbUser := cfgPkg.GetString("POSTGRES_USER", "postgres")
	dbPassword := cfgPkg.GetString("POSTGRES_PASSWORD", "postgres")
	dbHost := cfgPkg.GetString("POSTGRES_HOST", "postgres") // "postgres" in Docker, "localhost" locally
	dbPort := cfgPkg.GetString("POSTGRES_PORT", "5432")
	dbName := cfgPkg.GetString("POSTGRES_DB", "social")
	dbSSLMode := cfgPkg.GetString("POSTGRES_SSLMODE", "disable")

	dbAddr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSSLMode)

	cfg := config{
		addr: cfgPkg.GetString("ADDR", ":8080"),
		db: dbConfig{
			addr:         dbAddr,
			maxOpenConns: cfgPkg.GetInt("DB_MAX_OPEN_CONNS", 30),
			maxIdleConns: cfgPkg.GetInt("DB_MAX_IDLE_CONNS", 30),
			maxIdleTime:  cfgPkg.GetString("DB_MAX_IDLE_TIME", "15m"),
		},
	}

	logger.Logger.Info().
		Str("host", dbHost).
		Str("port", dbPort).
		Str("database", dbName).
		Str("sslmode", dbSSLMode).
		Msg("connecting to postgres")

	db, err := cfgPkg.NewDB(
		cfg.db.addr,
		cfg.db.maxOpenConns,
		cfg.db.maxIdleConns,
		cfg.db.maxIdleTime,
	)

	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("failed to connect to postgres")
	}
	defer db.Close()

	logger.Logger.Info().
		Str("host", dbHost).
		Str("database", dbName).
		Msg("postgres connection pool established")

	store := store.NewStorage(db)
	redisAddr := cfgPkg.GetString("REDIS_ADDR", "localhost:6379")
	redisDB := cfgPkg.GetInt("REDIS_DB", 0)

	logger.Logger.Info().
		Str("addr", redisAddr).
		Int("db", redisDB).
		Msg("connecting to redis")

	redisClient, err := cfgPkg.NewRedisClient()
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("failed to connect to redis")
	}
	defer redisClient.Close()

	logger.Logger.Info().
		Str("addr", redisAddr).
		Int("db", redisDB).
		Msg("redis connection established")

	// RabbitMQ connection (for publishing auth-related events, e.g., email verification)
	rabbitURL := cfgPkg.GetString("RABBITMQ_URL", "")
	if rabbitURL == "" {
		logger.Logger.Fatal().Msg("RABBITMQ_URL is required")
	}

	logger.Logger.Info().Str("url", rabbitURL).Msg("connecting to RabbitMQ")

	rabbitConn, rabbitCh, err := cfgPkg.NewRabbitMQConnection()
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
	defer rabbitConn.Close()
	defer rabbitCh.Close()

	logger.Logger.Info().Msg("RabbitMQ connection established")

	publisher := services.NewRabbitMQPublisher(rabbitCh)

	authService := services.NewAuthService(store, redisClient, publisher)

	app := &application{
		config:      cfg,
		store:       store,
		authService: authService,
		redisClient: redisClient,
		db:          db,
		rabbitConn:  rabbitConn,
		rabbitCh:    rabbitCh,
	}
	mux := app.mount()

	// Start server with graceful shutdown
	if err := app.runWithGracefulShutdown(mux, db, redisClient, rabbitConn, rabbitCh); err != nil {
		logger.Logger.Fatal().Err(err).Msg("server error")
	}
}

func validateRequiredEnv() error {
	if os.Getenv("JWT_SECRET") == "" {
		return fmt.Errorf("JWT_SECRET is required")
	}
	return nil
}
