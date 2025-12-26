package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"net/url"
	"time"

	_ "github.com/lib/pq"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/application/event"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/config"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/infrastructure/db/postgres"
	rabbitpub "github.com/baechuer/real-time-ressys/services/event-service/internal/infrastructure/messaging/rabbitmq"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/logger"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/handlers"
	authmw "github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/router"
	zlog "github.com/rs/zerolog/log"
)

// sysClock implements event.Clock interface using system time
type sysClock struct{}

func (sysClock) Now() time.Time { return time.Now().UTC() }

// App holds all dependencies for the service
type App struct {
	Config *config.Config
	Server *http.Server
	DB     *sql.DB

	Publisher *rabbitpub.Publisher
}

func main() {
	logger.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	u, _ := url.Parse(cfg.DatabaseURL)
	zlog.Info().
		Str("db_user", u.User.Username()).
		Str("db_host", u.Host).
		Str("db_db", u.Path).
		Msg("db config loaded")

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		zlog.Fatal().Err(err).Msg("db open failed")
	}
	defer db.Close()

	{
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			zlog.Fatal().Err(err).Msg("db ping failed")
		}
	}

	app := NewApp(cfg, db)
	defer func() {
		if app.Publisher != nil {
			_ = app.Publisher.Close()
		}
	}()

	zlog.Info().Str("addr", cfg.HTTPAddr).Msg("listening")
	if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zlog.Fatal().Err(err).Msg("server crashed")
	}
}

func NewApp(cfg *config.Config, db *sql.DB) *App {
	// 1) Infrastructure
	repo := postgres.New(db)

	// publisher wiring
	var rabbit *rabbitpub.Publisher
	var pub event.EventPublisher = event.NoopPublisher{}

	if cfg.RabbitURL != "" {
		p, err := rabbitpub.NewPublisher(cfg.RabbitURL, cfg.RabbitExchange)
		if err != nil {
			zlog.Fatal().Err(err).Msg("rabbit publisher init failed")
		}
		rabbit = p
		pub = p
		zlog.Info().Str("exchange", cfg.RabbitExchange).Msg("rabbit publisher ready")
	} else {
		zlog.Warn().Msg("RABBIT_URL empty: domain events will not be published")
	}

	// 2) Application
	svc := event.New(repo, sysClock{}, pub)

	// 3) Transport
	h := handlers.NewEventsHandler(svc, sysClock{})
	auth := authmw.NewAuth(cfg.JWTSecret, cfg.JWTIssuer)
	z := handlers.NewHealthHandler()

	// 4) Router
	httpHandler := router.New(h, auth, z)

	// 5) Server
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	return &App{
		Config:    cfg,
		Server:    srv,
		DB:        db,
		Publisher: rabbit,
	}
}
