package bootstrap

import (
	"context"
	"net/http"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/db/postgres"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/memory"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/security"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/logger"
	http_handlers "github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/handlers"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/middleware"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/response"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/transport/http/router"
)

func NewServer() (*http.Server, func(), error) {
	// 0) load config
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}

	// 1) init external deps (connections)
	db, err := config.NewDB(cfg.DBAddr, cfg.DBDebug)
	if err != nil {
		return nil, nil, err
	}

	// 2) build infra implementations (repos/stores/publishers)
	userRepo := postgres.NewUserRepo(db)

	// keep these in-memory for now
	sessionStore := memory.NewSessionStore()
	ottStore := memory.NewOneTimeTokenStore()
	publisher := memory.NewNoopPublisher()

	// 3) security
	hasher := security.NewBcryptHasher(12)
	signer := security.NewJWTSigner(cfg.JWTSecret, "auth-service")

	// ✅ seed initial users (dev only)
	if cfg.Env == "dev" {
		postgres.SeedUsers(context.Background(), userRepo, hasher)
	}

	// 4) usecase
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

	// 5) handlers
	secureCookies := cfg.Env != "dev"
	authH := http_handlers.NewAuthHandler(authSvc, cfg.RefreshTokenTTL, secureCookies)
	healthH := http_handlers.NewHealthHandler()

	authMW := middleware.Auth(signer, userRepo, response.WriteError)
	modMW := middleware.RequireAtLeast(string(domain.RoleModerator), response.WriteError)
	adminMW := middleware.RequireAtLeast("admin", response.WriteError)

	// 6) router
	mux, err := router.New(router.Deps{
		Auth:    authH,
		Health:  healthH,
		AuthMW:  authMW,
		ModMW:   modMW,
		AdminMW: adminMW,
	})
	if err != nil {
		_ = db.Close()
		return nil, nil, err
	}

	// 7) server
	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      mux,
		ReadTimeout:  cfg.HTTPReadTimeout,
		WriteTimeout: cfg.HTTPWriteTimeout,
		IdleTimeout:  cfg.HTTPIdleTimeout,
	}

	cleanup := func() {
		_ = db.Close()
	}

	return srv, cleanup, nil
}
