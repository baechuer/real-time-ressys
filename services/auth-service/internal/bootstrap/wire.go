package bootstrap

import (
	"context"
	"net/http"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/memory"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/security"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/logger"
	http_handlers "github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/handlers"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/response"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/router"
	// "auth-service/internal/infrastructure/db/postgres"
	// "auth-service/internal/infrastructure/messaging/rabbitmq"
	// "auth-service/internal/infrastructure/redis"
)

func NewServer() (*http.Server, func(), error) {
	// 0) load config
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	// // 1) init external deps (connections)
	// db, err := config.NewDB(cfg.DBAddr, cfg.DBMaxOpen, cfg.DBMaxIdle, cfg.DBMaxIdleTime)
	// if err != nil {
	// 	return nil, nil, err
	// }
	// In-memory adapters (MVP)
	userRepo := memory.NewUserRepo()
	sessionStore := memory.NewSessionStore()
	ottStore := memory.NewOneTimeTokenStore()
	publisher := memory.NewNoopPublisher()

	// Security
	hasher := security.NewBcryptHasher(12)
	signer := security.NewJWTSigner(cfg.JWTSecret, "auth-service")
	// ✅ seed initial users (dev only)
	if cfg.Env == "dev" {
		memory.SeedUsers(context.Background(), userRepo, hasher)
	}

	// rdb, err := redis.NewClient(cfg.RedisAddr) // 你实现：返回 *redis.Client 或 wrapper
	// if err != nil {
	// 	return nil, nil, err
	// }

	// pub, err := rabbitmq.NewPublisher(cfg.RabbitURL) // 你实现：连接 + channel
	// if err != nil {
	// 	return nil, nil, err
	// }

	// // 2) build infra implementations (repos/stores/publishers)
	// userRepo := postgres.NewUserRepo(db)
	// tokenStore := redis.NewTokenStore(rdb)
	// emailPublisher := rabbitmq.NewEmailPublisher(pub)

	// // 3) create usecase services
	// authSvc := auth.NewService(auth.Deps{
	// 	Users:  userRepo,
	// 	Tokens: tokenStore,
	// 	Email:  emailPublisher,
	// 	Clock:  time.Now, // 可选
	// })
	// Usecase service
	authSvc := auth.NewService(
		userRepo,
		hasher,
		signer,
		sessionStore,
		ottStore,
		publisher,
		auth.Config{
			AccessTTL:             cfg.AccessTokenTTL,
			RefreshTTL:            cfg.RefreshTokenTTL,
			VerifyEmailBaseURL:    cfg.VerifyEmailBaseURL,
			PasswordResetBaseURL:  cfg.PasswordResetBaseURL,
			VerifyEmailTokenTTL:   cfg.VerifyEmailTokenTTL,
			PasswordResetTokenTTL: cfg.PasswordResetTokenTTL,
		},
	)

	authSvc = authSvc.WithAudit(func(action string, fields map[string]string) {
		evt := logger.Logger.Info().
			Bool("audit", true).
			Str("action", action)

		for k, v := range fields {
			evt = evt.Str(k, v)
		}

		evt.Msg("audit")
	})
	logger.Logger.Info().Msg("audit sink installed")

	// // 4) handlers
	//authH := http_handlers.NewAuthHandler(authSvc)
	secureCookies := cfg.Env != "dev" // dev=false, staging/prod=true
	authH := http_handlers.NewAuthHandler(authSvc, cfg.RefreshTokenTTL, secureCookies)

	healthH := http_handlers.NewHealthHandler()

	authMW := middleware.Auth(signer, response.WriteError)
	modMW := middleware.RequireAtLeast(string(domain.RoleModerator), response.WriteError)
	adminMW := middleware.RequireAtLeast("admin", response.WriteError)

	// // 5) router
	mux, err := router.New(router.Deps{
		Auth:    authH,
		Health:  healthH,
		AuthMW:  authMW,
		ModMW:   modMW,
		AdminMW: adminMW,
	})

	if err != nil {
		return nil, nil, err
	}

	// 6) http server
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	// // 7) cleanup (important)
	cleanup := func() {
		// _ = pub.Close() // 你实现 Close
		// _ = rdb.Close() // redis client close
		// _ = db.Close()
	}

	// return srv, cleanup, nil
	return srv, cleanup, nil
}
