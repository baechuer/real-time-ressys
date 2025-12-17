package store

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

/*
Storage Test Cases:

1. TestNewStorage_Success
   - Creates Storage with UsersStore
   - Users field is not nil
   - Users is of type *UsersStore
*/

// TestNewStorage_Success tests successful storage creation
func TestNewStorage_Success(t *testing.T) {
	// Create a mock database connection using sqlmock
	// This doesn't require a real database connection
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock database: %v", err)
	}
	defer db.Close()

	storage := NewStorage(db)

	// Verify storage is created
	assert.NotNil(t, storage, "Storage should not be nil")
	assert.NotNil(t, storage.Users, "Users should not be nil")

	// Verify Users is of type *UsersStore
	_, ok := storage.Users.(*UsersStore)
	assert.True(t, ok, "Users should be of type *UsersStore")
}
