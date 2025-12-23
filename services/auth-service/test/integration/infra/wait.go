//go:build integration

package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	goredis "github.com/redis/go-redis/v9"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func WaitTCP(ctx context.Context, addr string) error {
	var last error
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait tcp: %w (last=%v)", ctx.Err(), last)
		case <-t.C:
			d := net.Dialer{Timeout: 800 * time.Millisecond}
			c, err := d.DialContext(ctx, "tcp", addr)
			if err != nil {
				last = err
				continue
			}
			_ = c.Close()
			return nil
		}
	}
}

func WaitPostgresDSN(ctx context.Context, dsn string) error {
	return WaitPostgres(ctx, dsn)
}

func WaitPostgres(ctx context.Context, dsn string) error {
	var last error
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait postgres: %w (last=%v)", ctx.Err(), last)
		case <-t.C:
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				last = err
				continue
			}
			err = db.PingContext(ctx)
			_ = db.Close()
			if err != nil {
				last = err
				continue
			}
			return nil
		}
	}
}

func WaitRedis(ctx context.Context, addr string) error {
	var last error
	t := time.NewTicker(300 * time.Millisecond)
	defer t.Stop()

	rdb := goredis.NewClient(&goredis.Options{Addr: addr})
	defer func() { _ = rdb.Close() }()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait redis: %w (last=%v)", ctx.Err(), last)
		case <-t.C:
			if err := rdb.Ping(ctx).Err(); err != nil {
				last = err
				continue
			}
			return nil
		}
	}
}

func WaitRabbit(ctx context.Context, amqpURL string) error {
	var last error
	t := time.NewTicker(400 * time.Millisecond)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("wait rabbit: %w (last=%v)", ctx.Err(), last)
		case <-t.C:
			conn, err := amqp.Dial(amqpURL)
			if err != nil {
				last = err
				continue
			}
			ch, err := conn.Channel()
			if err != nil {
				last = err
				_ = conn.Close()
				continue
			}
			// basic no-op
			_ = ch.Close()
			_ = conn.Close()

			if last != nil && errors.Is(last, amqp.ErrClosed) {
				// ignore
			}
			return nil
		}
	}
}
