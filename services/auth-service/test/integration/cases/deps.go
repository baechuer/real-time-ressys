//go:build integration

package cases

import (
	"context"
	"database/sql"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/application/auth"
	pg "github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/db/postgres"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/messaging/rabbitmq"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/infrastructure/security"
	itinfra "github.com/baechuer/real-time-ressys/services/auth-service/test/integration/infra"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Deps struct {
	DB   *sql.DB
	RDB  *goredis.Client
	AMQP *amqp.Connection

	Users    *pg.UserRepo
	Sessions auth.SessionStore
	Signer   auth.TokenSigner
	Hasher   auth.PasswordHasher
	Pub      auth.EventPublisher
	OTT      auth.OneTimeTokenStore

	Svc *auth.Service
}

func MustNewDeps(t *testing.T, env itinfra.Env) *Deps {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	require.NoError(t, itinfra.WaitPostgresDSN(ctx, env.PostgresDSN))
	require.NoError(t, itinfra.WaitRedis(ctx, env.RedisAddr))
	require.NoError(t, itinfra.WaitRabbit(ctx, env.RabbitURL))

	// --- Postgres ---
	db, err := sql.Open("pgx", env.PostgresDSN)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))
	require.NoError(t, itinfra.EnsureAuthSchema(ctx, db))

	// --- Redis ---
	rdb := goredis.NewClient(&goredis.Options{Addr: env.RedisAddr})
	require.NoError(t, rdb.Ping(ctx).Err())

	// --- RabbitMQ ---
	conn, err := amqp.Dial(env.RabbitURL)
	require.NoError(t, err)
	require.NoError(t, itinfra.EnsureRabbitTopology(ctx, env.RabbitURL))

	// publisher（你项目里的 mandatory+confirm 逻辑）
	pub, err := rabbitmq.NewPublisher(env.RabbitURL)
	require.NoError(t, err)

	// repos
	users := pg.NewUserRepo(db)

	// signer/hasher
	signer := security.NewJWTSigner("integration-test-secret", "auth-service-it")
	hasher := itinfra.NewBcryptHasherForIT(12)

	// sessions/ott
	sessions := itinfra.NewITRedisSessionStore(rdb)
	ott := itinfra.NewInMemoryOTT()

	cfg := auth.Config{
		AccessTTL:  15 * time.Minute,
		RefreshTTL: 7 * 24 * time.Hour,

		VerifyEmailBaseURL:    "https://frontend/verify-email?token=",
		PasswordResetBaseURL:  "https://frontend/reset-password?token=",
		VerifyEmailTokenTTL:   24 * time.Hour,
		PasswordResetTokenTTL: 30 * time.Minute,
	}

	svc := auth.NewService(users, hasher, signer, sessions, ott, pub, cfg)

	return &Deps{
		DB: db, RDB: rdb, AMQP: conn,
		Users:    users,
		Sessions: sessions,
		Signer:   signer,
		Hasher:   hasher,
		Pub:      pub,
		OTT:      ott,
		Svc:      svc,
	}
}

func (d *Deps) Close(t *testing.T) {
	t.Helper()
	if d.Pub != nil {
		if c, ok := d.Pub.(interface{ Close() error }); ok {
			_ = c.Close()
		}
	}
	if d.AMQP != nil {
		_ = d.AMQP.Close()
	}
	if d.RDB != nil {
		_ = d.RDB.Close()
	}
	if d.DB != nil {
		_ = d.DB.Close()
	}
}
