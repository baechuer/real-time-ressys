package bootstrap

import (
	"context"
	"net/http"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/config"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/db/postgres"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/memory"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/redis"
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

	// ---- Redis: token_version cache (best-effort) ----
	var redisClient *redis.Client

	// You currently require REDIS_ADDR in config.Load().
	// But at runtime Redis may still be temporarily unavailable.
	// In that case, we fall back to DB (cache disabled) instead of failing startup.
	{
		c := redis.New(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)

		// Use a short timeout on Ping so bootstrap isn't blocked.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if err := c.Ping(ctx); err != nil {
			logger.Logger.Warn().Err(err).Msg("redis unavailable; token_version cache disabled (DB fallback)")
			_ = c.Close()
		} else {
			redisClient = c
			logger.Logger.Info().Msg("redis connected")
		}
	}
	secureCookies := cfg.Env != "dev"

	// Wrap repo with cache if Redis is available
	var userRepoCached auth.UserRepo = userRepo
	if redisClient != nil {
		userRepoCached = redis.NewCachedUserRepo(userRepo, redisClient, cfg.TokenVersionCacheTTL)
	}
	// sessionStore: Redis preferred, fallback to memory (dev-friendly)
	var sessionStore auth.SessionStore
	if redisClient != nil {
		sessionStore = redis.NewRedisSessionStore(redisClient)
		logger.Logger.Info().Msg("session store: redis")
	} else {
		sessionStore = memory.NewSessionStore()
		logger.Logger.Warn().Msg("session store: memory (redis unavailable)")
	}
	// OneTimeTokenStore: Redis preferred, fallback memory
	var ottStore auth.OneTimeTokenStore
	if redisClient != nil {
		ottStore = redis.NewOneTimeTokenStore(redisClient)
		logger.Logger.Info().Msg("one-time-token store: redis")
	} else {
		ottStore = memory.NewOneTimeTokenStore()
		logger.Logger.Warn().Msg("one-time-token store: memory (redis unavailable)")
	}
	// keep these in-memory for now
	publisher := memory.NewNoopPublisher()

	// 3) security
	hasher := security.NewBcryptHasher(12)
	signer := security.NewJWTSigner(cfg.JWTSecret, "auth-service")

	// ✅ seed initial users (dev only)
	if cfg.Env == "dev" {
		// seed should hit DB directly; cache is irrelevant here
		postgres.SeedUsers(context.Background(), userRepo, hasher)
	}

	// 4) usecase
	authSvc := auth.NewService(
		userRepoCached, // ✅ important: cached repo here too (BumpTokenVersion updates cache)
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
	authH := http_handlers.NewAuthHandler(authSvc, cfg.RefreshTokenTTL, secureCookies)
	healthH := http_handlers.NewHealthHandler()

	authMW := middleware.Auth(signer, userRepoCached, response.WriteError)
	modMW := middleware.RequireAtLeast(string(domain.RoleModerator), response.WriteError)
	adminMW := middleware.RequireAtLeast("admin", response.WriteError)

	// ---- Rate Limiting (fixed window; best-effort) ----
	// If redisClient is nil (redis down), we disable rate limiting (fail-open).
	var fwLimiter *redis.FixedWindowLimiter
	if redisClient != nil {
		fwLimiter = redis.NewFixedWindowLimiter(redisClient)
	}

	// helper to create a middleware for a routeKey
	rl := func(routeKey string, limit int, window time.Duration) func(http.Handler) http.Handler {
		if fwLimiter == nil {
			return nil
		}
		return middleware.RateLimitFixedWindow(
			fwLimiter,
			middleware.FixedWindowConfig{
				RouteKey: routeKey,
				Limit:    limit,
				Window:   window,
			},
			response.WriteError,
		)
	}

	// Policy table (tune as needed)
	rlRegister := rl("auth.register", 3, time.Minute)                  // per IP
	rlLogin := rl("auth.login", 5, time.Minute)                        // per IP
	rlRefresh := rl("auth.refresh", 10, time.Minute)                   // per user (or IP if anonymous)
	rlLogout := rl("auth.logout", 30, time.Minute)                     // per user
	rlVerifyReq := rl("auth.verify_email.request", 3, 10*time.Minute)  // per IP
	rlResetReq := rl("auth.password_reset.request", 3, 10*time.Minute) // per IP

	rlPwdChange := rl("auth.password.change", 5, time.Minute)  // per user
	rlSessRevoke := rl("auth.sessions.revoke", 5, time.Minute) // per user
	rlMod := rl("auth.mod.actions", 30, time.Minute)           // per actor
	rlAdmin := rl("auth.admin.actions", 60, time.Minute)       // per actor

	// 6) router
	mux, err := router.New(router.Deps{
		Auth:    authH,
		Health:  healthH,
		AuthMW:  authMW,
		ModMW:   modMW,
		AdminMW: adminMW,

		RLRegister:             rlRegister,
		RLLogin:                rlLogin,
		RLRefresh:              rlRefresh,
		RLLogout:               rlLogout,
		RLVerifyEmailRequest:   rlVerifyReq,
		RLPasswordResetRequest: rlResetReq,
		RLPasswordChange:       rlPwdChange,
		RLSessionsRevoke:       rlSessRevoke,
		RLModActions:           rlMod,
		RLAdminActions:         rlAdmin,
	})

	if err != nil {
		_ = db.Close()
		if redisClient != nil {
			_ = redisClient.Close()
		}
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
		if redisClient != nil {
			_ = redisClient.Close()
		}
	}

	return srv, cleanup, nil
}
