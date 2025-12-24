package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/baechuer/real-time-ressys/services/email-service/internal/bootstrap"
	"github.com/baechuer/real-time-ressys/services/email-service/internal/logger"
)

// runner abstracts the application lifecycle.
// Start launches the service (may block or spawn goroutines).
// Stop performs a graceful shutdown.
type runner interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// builder constructs the application instance and returns a cleanup function.
// The cleanup function is responsible for releasing resources.
type builder func() (runner, func(), error)

// Run is a generic service runner that:
// 1. Bootstraps the application
// 2. Starts it asynchronously
// 3. Listens for OS shutdown signals or runtime crashes
// 4. Performs a graceful shutdown with timeout
//
// It returns a process exit code.
func Run(build builder, sigCh <-chan os.Signal, lg zerolog.Logger) int {
	app, cleanup, err := build()
	if err != nil {
		lg.Error().Err(err).Msg("bootstrap failed")
		return 1
	}
	defer cleanup()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		lg.Info().Msg("email-service starting")
		if err := app.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
		}
	}()

	select {
	case sig := <-sigCh:
		lg.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case err := <-errCh:
		lg.Error().Err(err).Msg("app crashed")
		return 1
	}

	// Graceful shutdown with timeout
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer stopCancel()

	if err := app.Stop(stopCtx); err != nil {
		lg.Error().Err(err).Msg("graceful stop failed")
		return 1
	}

	lg.Info().Msg("shutdown complete")
	return 0
}

// buildFromBootstrap wires the application using the bootstrap package.
// It adapts the bootstrap output to the generic runner interface.
func buildFromBootstrap() (runner, func(), error) {
	app, cleanup, err := bootstrap.NewApp()
	if err != nil {
		return nil, nil, err
	}
	return app, cleanup, nil
}

func main() {
	// If you already have a shared logger initializer in auth-service,
	// this can be reused here. For now, use zerolog defaults.

	logger.Init()

	zerolog.TimeFieldFormat = time.RFC3339Nano

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	code := Run(buildFromBootstrap, sigCh, zlog.Logger)
	os.Exit(code)
}
