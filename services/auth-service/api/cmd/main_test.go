package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/rs/zerolog"
)

type fakeServer struct {
	addr string

	listenErr   error
	shutdownErr error
	closeErr    error

	listenCalled   bool
	shutdownCalled bool
	closeCalled    bool
}

func (f *fakeServer) ListenAndServe() error {
	f.listenCalled = true
	return f.listenErr
}
func (f *fakeServer) Shutdown(ctx context.Context) error {
	f.shutdownCalled = true
	return f.shutdownErr
}
func (f *fakeServer) Close() error {
	f.closeCalled = true
	return f.closeErr
}
func (f *fakeServer) Addr() string { return f.addr }

func TestRun_BootstrapFail_Returns1(t *testing.T) {
	lg := zerolog.Nop()
	sigCh := make(chan os.Signal, 1)

	build := func() (httpServer, func(), error) {
		return nil, func() {}, errors.New("boom")
	}

	if got := Run(build, sigCh, lg); got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
}

func TestRun_OnSignal_ShutdownAndReturn0(t *testing.T) {
	lg := zerolog.Nop()

	// Pre-send a signal so Run() will take the signal path deterministically.
	sigCh := make(chan os.Signal, 1)
	sigCh <- os.Interrupt

	fs := &fakeServer{
		addr:      ":0",
		listenErr: http.ErrServerClosed, // ListenAndServe returns this on normal shutdown
	}

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return fs, func() { cleanupCalled = true }, nil
	}

	got := Run(build, sigCh, lg)

	if got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
	if !fs.listenCalled {
		t.Fatalf("expected ListenAndServe called")
	}
	if !fs.shutdownCalled {
		t.Fatalf("expected Shutdown called")
	}
	if fs.closeCalled {
		t.Fatalf("did not expect Close called on graceful shutdown")
	}
	if !cleanupCalled {
		t.Fatalf("expected cleanup called")
	}
}

func TestRun_OnServerCrash_Return1(t *testing.T) {
	lg := zerolog.Nop()
	sigCh := make(chan os.Signal, 1)

	fs := &fakeServer{
		addr:      ":0",
		listenErr: errors.New("crash"),
	}

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return fs, func() { cleanupCalled = true }, nil
	}

	got := Run(build, sigCh, lg)

	if got != 1 {
		t.Fatalf("expected 1, got %d", got)
	}
	if !fs.listenCalled {
		t.Fatalf("expected ListenAndServe called")
	}
	// crash path does not call Shutdown/Close in current Run() design
	if fs.shutdownCalled {
		t.Fatalf("did not expect Shutdown called on crash path")
	}
	if !cleanupCalled {
		t.Fatalf("expected cleanup called")
	}
}

func TestRun_ShutdownFail_ForcesClose(t *testing.T) {
	lg := zerolog.Nop()

	sigCh := make(chan os.Signal, 1)
	sigCh <- os.Interrupt

	fs := &fakeServer{
		addr:        ":0",
		listenErr:   http.ErrServerClosed,
		shutdownErr: errors.New("shutdown failed"),
	}

	build := func() (httpServer, func(), error) {
		return fs, func() {}, nil
	}

	_ = Run(build, sigCh, lg)

	if !fs.shutdownCalled {
		t.Fatalf("expected Shutdown called")
	}
	if !fs.closeCalled {
		t.Fatalf("expected Close called when Shutdown fails")
	}
}
