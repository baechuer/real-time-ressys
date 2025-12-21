// api/cmd/main_test.go
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

type fakeServer struct {
	addr string

	mu sync.Mutex

	startedCh chan struct{} // closed when ListenAndServe starts
	stopCh    chan struct{} // closed to unblock ListenAndServe
	doneCh    chan struct{} // closed when ListenAndServe exits

	// Behavior controls
	listenErr error // if set, ListenAndServe returns this error immediately

	// Observability
	shutdownCalled bool
	closeCalled    bool
	shutdownCtx    context.Context
}

func newFakeServer(addr string) *fakeServer {
	return &fakeServer{
		addr:      addr,
		startedCh: make(chan struct{}),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

func (f *fakeServer) Addr() string { return f.addr }

func (f *fakeServer) ListenAndServe() error {
	// Mark started.
	select {
	case <-f.startedCh:
		// already closed
	default:
		close(f.startedCh)
	}

	// Crash immediately if configured.
	if f.listenErr != nil {
		close(f.doneCh)
		return f.listenErr
	}

	// Otherwise block until stopped, then behave like a normal server shutdown.
	<-f.stopCh
	close(f.doneCh)
	return http.ErrServerClosed
}

func (f *fakeServer) Shutdown(ctx context.Context) error {
	f.mu.Lock()
	f.shutdownCalled = true
	f.shutdownCtx = ctx
	f.mu.Unlock()

	// Unblock ListenAndServe
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}
	return nil
}

func (f *fakeServer) Close() error {
	f.mu.Lock()
	f.closeCalled = true
	f.mu.Unlock()

	// Unblock ListenAndServe if still running
	select {
	case <-f.stopCh:
	default:
		close(f.stopCh)
	}

	return nil
}

func (f *fakeServer) waitStarted(t *testing.T) {
	t.Helper()
	select {
	case <-f.startedCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ListenAndServe did not start in time")
	}
}

func (f *fakeServer) waitDone(t *testing.T) {
	t.Helper()
	select {
	case <-f.doneCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("ListenAndServe did not exit in time")
	}
}

func nopLogger() zerolog.Logger {
	// Disable output; we only care about control flow.
	return zerolog.New(os.Stdout).Level(zerolog.Disabled)
}

func TestRun_BuildFails_Returns1_NoCleanup(t *testing.T) {
	t.Parallel()

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return nil, func() { cleanupCalled = true }, errors.New("boom")
	}

	sigCh := make(chan os.Signal, 1)
	code := Run(build, sigCh, nopLogger())

	if code != 1 {
		t.Fatalf("expected code=1, got %d", code)
	}
	if cleanupCalled {
		t.Fatalf("cleanup should NOT be called when build fails")
	}
}

func TestRun_Signal_ShutsDown_Returns0_CallsCleanup(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(":1234")

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return fs, func() { cleanupCalled = true }, nil
	}

	sigCh := make(chan os.Signal, 1)

	// Run in goroutine so we can send signal after server starts.
	done := make(chan int, 1)
	go func() {
		done <- Run(build, sigCh, nopLogger())
	}()

	// Ensure server started listening.
	fs.waitStarted(t)

	// Trigger shutdown.
	sigCh <- os.Interrupt

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected code=0, got %d", code)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Run did not return in time")
	}

	// Verify graceful shutdown path.
	fs.mu.Lock()
	shutdownCalled := fs.shutdownCalled
	closeCalled := fs.closeCalled
	ctx := fs.shutdownCtx
	fs.mu.Unlock()

	if !shutdownCalled {
		t.Fatalf("expected Shutdown to be called")
	}
	if closeCalled {
		t.Fatalf("did not expect Close to be called on successful Shutdown")
	}
	if ctx == nil {
		t.Fatalf("expected shutdown context to be set")
	}
	if cleanupCalled != true {
		t.Fatalf("expected cleanup to be called")
	}

	// Ensure server goroutine exits.
	fs.waitDone(t)
}

func TestRun_ServerCrash_Returns1_CallsCleanup(t *testing.T) {
	t.Parallel()

	fs := newFakeServer(":1234")
	fs.listenErr = errors.New("listen failed")

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return fs, func() { cleanupCalled = true }, nil
	}

	sigCh := make(chan os.Signal, 1)
	code := Run(build, sigCh, nopLogger())

	if code != 1 {
		t.Fatalf("expected code=1, got %d", code)
	}
	if !cleanupCalled {
		t.Fatalf("expected cleanup to be called")
	}

	// ListenAndServe should have exited.
	fs.waitDone(t)
}

// ---- explicit server for shutdown-fail case ----

type shutdownFailingServer struct {
	*fakeServer
	shutdownErr error
}

func (s *shutdownFailingServer) Shutdown(ctx context.Context) error {
	s.fakeServer.mu.Lock()
	s.fakeServer.shutdownCalled = true
	s.fakeServer.shutdownCtx = ctx
	s.fakeServer.mu.Unlock()

	// Unblock ListenAndServe
	select {
	case <-s.fakeServer.stopCh:
	default:
		close(s.fakeServer.stopCh)
	}
	return s.shutdownErr
}

func TestRun_ShutdownFails_ForceClose_Returns0(t *testing.T) {
	t.Parallel()

	base := newFakeServer(":1234")
	fs := &shutdownFailingServer{
		fakeServer:  base,
		shutdownErr: errors.New("shutdown failed"),
	}

	cleanupCalled := false
	build := func() (httpServer, func(), error) {
		return fs, func() { cleanupCalled = true }, nil
	}

	sigCh := make(chan os.Signal, 1)

	done := make(chan int, 1)
	go func() { done <- Run(build, sigCh, nopLogger()) }()

	base.waitStarted(t)
	sigCh <- os.Interrupt

	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected code=0, got %d", code)
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("Run did not return in time")
	}

	base.mu.Lock()
	shutdownCalled := base.shutdownCalled
	closeCalled := base.closeCalled
	base.mu.Unlock()

	if !shutdownCalled {
		t.Fatalf("expected Shutdown to be called")
	}
	if !closeCalled {
		t.Fatalf("expected Close to be called when Shutdown fails")
	}
	if !cleanupCalled {
		t.Fatalf("expected cleanup to be called")
	}

	base.waitDone(t)
}
