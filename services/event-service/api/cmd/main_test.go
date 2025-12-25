package main

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/baechuer/real-time-ressys/services/event-service/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNewApp(t *testing.T) {
	// Setup mock DB to avoid real connection
	db, _, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	// Mock Config
	cfg := &config.Config{
		HTTPAddr:  ":8081",
		JWTSecret: "test-secret",
		JWTIssuer: "test-issuer",
	}

	t.Run("should_correctly_wire_dependencies", func(t *testing.T) {
		app := NewApp(cfg, db)

		assert.NotNil(t, app)
		assert.Equal(t, cfg.HTTPAddr, app.Server.Addr)
		assert.NotNil(t, app.Server.Handler, "HTTP Handler should be initialized")
	})
}

func TestSysClock_Now(t *testing.T) {
	clock := sysClock{}
	now := clock.Now()

	// Verify the clock uses UTC as per requirements
	assert.Equal(t, "UTC", now.Location().String())
}
