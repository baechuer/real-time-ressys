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
	"github.com/baechuer/real-time-ressys/services/event-service/internal/infrastructure/caching/redis"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/infrastructure/db/postgres"
	rabbitpub "github.com/baechuer/real-time-ressys/services/event-service/internal/infrastructure/messaging/rabbitmq"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/logger"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/handlers"
	authmw "github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/middleware"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/transport/http/router"
	go_redis "github.com/redis/go-redis/v9"
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

	Publisher  *rabbitpub.Publisher
	RedisCache *redis.Client
}

func main() {
	// 1. Load Config first (so .env is loaded)
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// 2. Initialize Logger (now it can see LOG_LEVEL from .env)
	logger.Init()

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
		if app.RedisCache != nil {
			_ = app.RedisCache.Close()
		}
	}()

	zlog.Info().Str("addr", cfg.HTTPAddr).Msg("listening")
	if err := app.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		zlog.Fatal().Err(err).Msg("server crashed")
	}
}

// ... 前面部分保持不变 ...

func NewApp(cfg *config.Config, db *sql.DB) *App {
	repo := postgres.New(db)

	var rabbit *rabbitpub.Publisher
	var pub event.EventPublisher = event.NoopPublisher{}

	if cfg.RabbitURL != "" {
		p, err := rabbitpub.NewPublisher(cfg.RabbitURL, cfg.RabbitExchange)
		if err != nil {
			zlog.Fatal().Err(err).Msg("rabbit publisher init failed")
		}
		rabbit = p
		pub = p
	}

	// redis wiring
	var rc *redis.Client // 我们的包装器
	var cache event.Cache
	var rawRedisInstance *go_redis.Client // 官方实例

	if cfg.RedisURL != "" {
		c, err := redis.New(cfg.RedisURL) // 调用自定义包
		if err != nil {
			zlog.Warn().Err(err).Msg("redis connect failed")
		} else {
			rc = c
			cache = c
			rawRedisInstance = c.GetRawClient() // 获取官方底层实例
			zlog.Info().Msg("redis cache ready")
		}
	}

	svc := event.New(repo, sysClock{}, pub, cache, cfg.CacheTTLDetails, cfg.CacheTTLList)
	h := handlers.NewEventsHandler(svc, sysClock{})
	auth := authmw.NewAuth(cfg.JWTSecret, cfg.JWTIssuer)
	z := handlers.NewHealthHandler()

	// 注入官方 Redis 实例供限流使用
	httpHandler := router.New(h, auth, z, rawRedisInstance, cfg)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      httpHandler,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	return &App{
		Config:     cfg,
		Server:     srv,
		DB:         db,
		Publisher:  rabbit,
		RedisCache: rc,
	}
}
