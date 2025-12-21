// api/cmd/main.go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/bootstrap"
	"github.com/baechuer/real-time-ressys/services/auth-service/internal/logger"
)

// httpServer defines the minimal surface area Run() needs from an HTTP server.
// Using an interface makes Run() easy to unit-test with a fake implementation.
type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
	Close() error
	Addr() string
}

// realServer adapts *http.Server to the httpServer interface.
type realServer struct{ *http.Server }

// Addr returns the configured listen address.
func (r realServer) Addr() string { return r.Server.Addr }

// serverBuilder builds the server and returns a cleanup function.
// In production we wrap bootstrap.NewServer(); in tests we can inject a fake.
type serverBuilder func() (httpServer, func(), error)

func Run(build serverBuilder, sigCh <-chan os.Signal, lg zerolog.Logger) int {
	srv, cleanup, err := build()
	if err != nil {
		lg.Error().Err(err).Msg("bootstrap failed")
		return 1
	}
	defer cleanup()

	// Start the HTTP server in a goroutine (ListenAndServe blocks).
	errCh := make(chan error, 1)
	go func() {
		lg.Info().Str("addr", srv.Addr()).Msg("listening")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Wait for either an OS shutdown signal or a server crash.
	select {
	case sig := <-sigCh:
		lg.Info().Str("signal", sig.String()).Msg("shutdown signal received")

	case err := <-errCh:
		// Server crashed unexpectedly; exit non-zero so an orchestrator can restart it.
		lg.Error().Err(err).Msg("server crashed")
		return 1
	}

	// Attempt a graceful shutdown with a timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		// Graceful shutdown failed; force close the server.
		lg.Error().Err(err).Msg("graceful shutdown failed")
		_ = srv.Close()
	}

	lg.Info().Msg("shutdown complete")
	return 0
}

// buildFromBootstrap wraps bootstrap.NewServer() into a serverBuilder.
func buildFromBootstrap() (httpServer, func(), error) {
	srv, cleanup, err := bootstrap.NewServer()
	if err != nil {
		return nil, nil, err
	}
	return realServer{srv}, cleanup, nil
}

func main() {
	// Initialize zerolog global config and our logger defaults.
	logger.Init()

	// Set up OS signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// Run the server and exit with the returned code.
	code := Run(buildFromBootstrap, sigCh, zlog.Logger)
	os.Exit(code)
}
