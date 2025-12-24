package bootstrap

import (
	"context"
	"strings"

	"github.com/gomodule/redigo/redis"
	"github.com/rs/zerolog/log"

	"github.com/baechuer/real-time-ressys/services/email-service/internal/application/notify"
	"github.com/baechuer/real-time-ressys/services/email-service/internal/config"
	infraemail "github.com/baechuer/real-time-ressys/services/email-service/internal/infrastructure/email"
	"github.com/baechuer/real-time-ressys/services/email-service/internal/infrastructure/idempotency"
	rmq "github.com/baechuer/real-time-ressys/services/email-service/internal/infrastructure/messaging/rabbitmq"
	web "github.com/baechuer/real-time-ressys/services/email-service/internal/infrastructure/web"
)

type App struct {
	consumer *rmq.Consumer
	web      *web.Server
	cfg      *config.Config
}

func NewApp() (*App, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	keys := splitCSV(cfg.BindKeysCSV)

	// Sender
	var sender notify.Sender
	switch cfg.EmailSender {
	case "smtp":
		sender = infraemail.NewSMTPSender(infraemail.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			Timeout:  cfg.SMTPTimeout,
		}, log.Logger)
	default:
		sender = infraemail.NewFakeSender(log.Logger)
	}

	// Redis pool (shared) + idempotency store
	var redisPool *redis.Pool
	var idem notify.IdempotencyStore
	var poolCleanup func()

	if cfg.RedisEnabled {
		redisPool = idempotency.NewRedisPool(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
		idem = idempotency.NewRedisStore(redisPool, log.Logger)
		poolCleanup = func() { _ = redisPool.Close() }

		log.Info().
			Str("addr", cfg.RedisAddr).
			Int("db", cfg.RedisDB).
			Msg("redis enabled for idempotency + http rate limit")
	} else {
		log.Info().Msg("redis disabled (idempotency + http rate limit)")
	}

	notifySvc := notify.NewService(sender, idem, cfg.EmailIdempotencyTTL, log.Logger)

	// Rabbit consumer
	consumer := rmq.NewConsumer(rmq.Config{
		RabbitURL:          cfg.RabbitURL,
		Exchange:           cfg.Exchange,
		Queue:              cfg.Queue,
		BindKeys:           keys,
		Prefetch:           cfg.Prefetch,
		Tag:                cfg.ConsumeTag,
		EmailPublicBaseURL: cfg.EmailPublicBaseURL,
	}, notifySvc, log.Logger)

	// Web server (8090) + Redis RL
	webSrv := web.NewServer(web.Config{
		Addr:      cfg.EmailWebAddr,
		AuthBase:  cfg.AuthBaseURL,
		RedisPool: redisPool, // ✅ shared pool

		RateLimit: web.RateLimitConfig{
			Enabled:     cfg.RLEnabled,
			IPLimit:     cfg.RLIPLimit,
			IPWindow:    cfg.RLIPWindow,
			TokenLimit:  cfg.RLTokenLimit,
			TokenWindow: cfg.RLTokenWindow,
		},
	}, log.Logger)

	app := &App{
		consumer: consumer,
		web:      webSrv,
		cfg:      cfg,
	}

	cleanup := func() {
		log.Info().Msg("Performing final resource cleanup...")

		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownWait)
		defer cancel()

		_ = app.Stop(ctx)
		if poolCleanup != nil {
			poolCleanup()
		}
	}

	return app, cleanup, nil
}

func (a *App) Start(ctx context.Context) error {
	log.Info().Msg("Starting Email Service consumer...")
	if err := a.consumer.Start(ctx); err != nil {
		return err
	}
	log.Info().Msg("Starting Email Service web...")
	return a.web.Start(ctx) // block
}

func (a *App) Stop(ctx context.Context) error {
	log.Info().Msg("Shutting down Email Service gracefully...")

	if a.web != nil {
		_ = a.web.Stop(ctx)
	}
	if a.consumer != nil {
		_ = a.consumer.Stop(ctx)
	}
	return nil
}

func splitCSV(s string) []string {
	raw := strings.Split(s, ",")
	out := make([]string, 0, len(raw))
	for _, x := range raw {
		x = strings.TrimSpace(x)
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
