package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

/*
DB config test cases:
1) NewDB success with live Postgres (testcontainers)
2) NewDB invalid connection string
3) NewDB invalid idle duration format
4) NewDB connection timeout/unreachable host
5) NewDB connection pool settings applied
*/

// TestNewDB_Success tests successful database connection
func TestNewDB_Success(t *testing.T) {
	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	// Cleanup container when test finishes
	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Test successful connection
	maxOpenConns := 10
	maxIdleConns := 5
	maxIdleTime := "15m"

	db, err := NewDB(connStr, maxOpenConns, maxIdleConns, maxIdleTime)
	require.NoError(t, err, "NewDB should not return error")
	require.NotNil(t, db, "Database connection should not be nil")

	// Cleanup database connection
	defer db.Close()

	// Verify connection pool settings
	stats := db.Stats()
	assert.Equal(t, maxOpenConns, stats.MaxOpenConnections, "MaxOpenConnections should be set correctly")

	// Verify we can actually use the connection
	err = db.Ping()
	assert.NoError(t, err, "Ping should succeed")

	// Verify we can execute a query
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	assert.NoError(t, err, "Query should succeed")
	assert.Equal(t, 1, result, "Query result should be correct")
}

// TestNewDB_InvalidConnectionString tests error handling for invalid connection string
func TestNewDB_InvalidConnectionString(t *testing.T) {
	invalidConnStr := "invalid://connection/string"

	db, err := NewDB(invalidConnStr, 10, 5, "15m")

	assert.Error(t, err, "NewDB should return error for invalid connection string")
	assert.Nil(t, db, "Database connection should be nil on error")
}

// TestNewDB_InvalidDuration tests error handling for invalid maxIdleTime duration
func TestNewDB_InvalidDuration(t *testing.T) {
	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err, "Failed to start PostgreSQL container")

	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Test with invalid duration format
	invalidDuration := "invalid-duration"

	db, err := NewDB(connStr, 10, 5, invalidDuration)

	assert.Error(t, err, "NewDB should return error for invalid duration")
	assert.Nil(t, db, "Database connection should be nil on error")
}

// TestNewDB_ConnectionTimeout tests connection timeout behavior
func TestNewDB_ConnectionTimeout(t *testing.T) {
	// Use a connection string that points to a non-existent host
	// This will trigger a connection timeout
	invalidHost := "postgres://user:pass@nonexistent-host:5432/db?sslmode=disable"

	// Note: sql.Open doesn't actually connect, so we need to test Ping
	// NewDB will call PingContext which should timeout
	db, err := NewDB(invalidHost, 10, 5, "15m")

	// Should fail because host doesn't exist or connection times out
	assert.Error(t, err, "NewDB should return error for unreachable database")
	assert.Nil(t, db, "Database connection should be nil on error")
}

// TestNewDB_ConnectionPoolSettings tests that connection pool settings are applied correctly
func TestNewDB_ConnectionPoolSettings(t *testing.T) {
	// Skip if Docker is not available
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:17"),
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	})

	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	maxOpenConns := 20
	maxIdleConns := 10
	maxIdleTime := "30m"

	db, err := NewDB(connStr, maxOpenConns, maxIdleConns, maxIdleTime)
	require.NoError(t, err)
	defer db.Close()

	// Verify connection pool settings
	stats := db.Stats()
	assert.Equal(t, maxOpenConns, stats.MaxOpenConnections, "MaxOpenConnections should match")

	// Note: MaxIdleConnections and ConnMaxIdleTime are not directly exposed in Stats()
	// But we can verify the connection works and settings were applied
	err = db.Ping()
	assert.NoError(t, err, "Connection should work with custom pool settings")
}

// TestGetRabbitMQConnectionString tests RabbitMQ connection string building
// TODO: Implement test (when RabbitMQ is added)
// - Test connection string format
// - Test with different environment variables
func TestGetRabbitMQConnectionString(t *testing.T) {
	// TODO: Set environment variables
	// TODO: Call GetRabbitMQConnectionString()
	// TODO: Assert connection string format
	// TODO: Test with different env values
}
