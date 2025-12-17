package store

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/baechuer/real-time-ressys/services/auth-service/app/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
UsersStore Test Cases:

1. TestUsersStore_Create_Success
   - Successful user creation
   - ID and CreatedAt are set
   - All fields are saved correctly

2. TestUsersStore_Create_DatabaseError
   - Database error during insert
   - Error is returned

3. TestUsersStore_GetByEmail_Success
   - User found by email
   - All fields are returned correctly

4. TestUsersStore_GetByEmail_NotFound
   - User not found (sql.ErrNoRows)
   - Error is returned

5. TestUsersStore_GetByEmail_DatabaseError
   - Database error during query
   - Error is returned

6. TestUsersStore_GetByID_Success
   - User found by ID
   - All fields are returned correctly

7. TestUsersStore_GetByID_NotFound
   - User not found (sql.ErrNoRows)
   - Error is returned

8. TestUsersStore_GetAll_Success
   - Multiple users returned (stub placeholder)
   - All fields are correct (to be asserted when implemented)

9. TestUsersStore_Update_Success
   - User updated successfully (stub placeholder)
   - Fields are updated correctly (to be asserted when implemented)

10. TestUsersStore_Delete_Success
    - User deleted successfully (stub placeholder)
    - User cannot be retrieved after deletion (to be asserted when implemented)

11. TestUsersStore_Create_ScanError
    - Insert returns malformed row -> scan fails -> error returned

12. TestUsersStore_GetByEmail_ScanError
    - Select returns malformed row -> scan fails -> error returned
*/

// setupMockDB creates a mock database and UsersStore for testing
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *UsersStore) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err, "Failed to create mock database")

	store := &UsersStore{db: db}

	return db, mock, store
}

// TestUsersStore_Create_Success tests successful user creation
func TestUsersStore_Create_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	user := &models.User{
		Username:        "testuser",
		Email:           "test@example.com",
		PasswordHash:    "$2a$10$hashedpassword",
		IsEmailVerified: false,
	}

	expectedID := int64(1)
	expectedCreatedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	// Expect INSERT query
	mock.ExpectQuery(`INSERT INTO users \(username, email, password_hash, is_email_verified\)
	VALUES \(\$1, \$2, \$3, \$4\)
	RETURNING id, created_at`).
		WithArgs(user.Username, user.Email, user.PasswordHash, user.IsEmailVerified).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(expectedID, expectedCreatedAt))

	err := store.Create(context.Background(), user)

	// Assertions
	require.NoError(t, err, "Create should not return error")
	assert.Equal(t, expectedID, user.ID, "User ID should be set")
	assert.Equal(t, expectedCreatedAt, user.CreatedAt, "CreatedAt should be set")
	assert.NoError(t, mock.ExpectationsWereMet(), "All expectations should be met")
}

// TestUsersStore_Create_DatabaseError tests database error during creation
func TestUsersStore_Create_DatabaseError(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	user := &models.User{
		Username:        "testuser",
		Email:           "test@example.com",
		PasswordHash:    "$2a$10$hashedpassword",
		IsEmailVerified: false,
	}

	// Expect INSERT query to fail
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(user.Username, user.Email, user.PasswordHash, user.IsEmailVerified).
		WillReturnError(sql.ErrConnDone)

	err := store.Create(context.Background(), user)

	// Assertions
	assert.Error(t, err, "Create should return error")
	assert.True(t, err == sql.ErrConnDone, "Error should be connection done")
	assert.NoError(t, mock.ExpectationsWereMet(), "All expectations should be met")
}

// TestUsersStore_GetByEmail_Success tests successful user retrieval by email
func TestUsersStore_GetByEmail_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	email := "test@example.com"
	expectedUser := &models.User{
		ID:              1,
		Username:        "testuser",
		Email:           email,
		PasswordHash:    "$2a$10$hashedpassword",
		IsEmailVerified: true,
		RoleID:          1,
		CreatedAt:       time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Expect SELECT query
	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_email_verified", "role_id", "created_at"}).
			AddRow(expectedUser.ID, expectedUser.Username, expectedUser.Email, expectedUser.PasswordHash, expectedUser.IsEmailVerified, expectedUser.RoleID, expectedUser.CreatedAt))

	user, err := store.GetByEmail(context.Background(), email)

	// Assertions
	require.NoError(t, err, "GetByEmail should not return error")
	require.NotNil(t, user, "User should not be nil")
	assert.Equal(t, expectedUser.ID, user.ID)
	assert.Equal(t, expectedUser.Username, user.Username)
	assert.Equal(t, expectedUser.Email, user.Email)
	assert.Equal(t, expectedUser.PasswordHash, user.PasswordHash)
	assert.Equal(t, expectedUser.IsEmailVerified, user.IsEmailVerified)
	assert.Equal(t, expectedUser.CreatedAt, user.CreatedAt)
	assert.NoError(t, mock.ExpectationsWereMet(), "All expectations should be met")
}

// TestUsersStore_GetByEmail_NotFound tests user not found scenario
func TestUsersStore_GetByEmail_NotFound(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	email := "nonexistent@example.com"

	// Expect SELECT query to return no rows
	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnError(sql.ErrNoRows)

	user, err := store.GetByEmail(context.Background(), email)

	// Assertions
	assert.Error(t, err, "GetByEmail should return error")
	assert.True(t, err == sql.ErrNoRows, "Error should be sql.ErrNoRows")
	assert.Nil(t, user, "User should be nil")
	assert.NoError(t, mock.ExpectationsWereMet(), "All expectations should be met")
}

// TestUsersStore_GetByEmail_DatabaseError tests database error during query
func TestUsersStore_GetByEmail_DatabaseError(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	email := "test@example.com"

	// Expect SELECT query to fail
	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnError(sql.ErrConnDone)

	user, err := store.GetByEmail(context.Background(), email)

	// Assertions
	assert.Error(t, err, "GetByEmail should return error")
	assert.True(t, err == sql.ErrConnDone, "Error should be connection done")
	assert.Nil(t, user, "User should be nil")
	assert.NoError(t, mock.ExpectationsWereMet(), "All expectations should be met")
}

// TestUsersStore_GetByID_Success tests successful user retrieval by ID
func TestUsersStore_GetByID_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	id := "1"

	expectedUser := &models.User{
		ID:              1,
		Username:        "testuser",
		Email:           "test@example.com",
		PasswordHash:    "$2a$10$hashedpassword",
		IsEmailVerified: true,
		RoleID:          1,
		CreatedAt:       time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE id = \$1`).
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_email_verified", "role_id", "created_at"}).
			AddRow(expectedUser.ID, expectedUser.Username, expectedUser.Email, expectedUser.PasswordHash, expectedUser.IsEmailVerified, expectedUser.RoleID, expectedUser.CreatedAt))

	user, err := store.GetByID(context.Background(), id)
	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, expectedUser.ID, user.ID)
	assert.Equal(t, expectedUser.Username, user.Username)
	assert.Equal(t, expectedUser.Email, user.Email)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_GetByID_NotFound tests user not found by ID
func TestUsersStore_GetByID_NotFound(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	id := "999"

	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE id = \$1`).
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	user, err := store.GetByID(context.Background(), id)
	assert.Error(t, err)
	assert.True(t, err == sql.ErrNoRows)
	assert.Nil(t, user)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_GetAll_Success tests successful retrieval of all users
func TestUsersStore_GetAll_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	expectedUsers := []models.User{
		{ID: 1, Username: "user1", Email: "user1@example.com", PasswordHash: "hash1", IsEmailVerified: true, RoleID: 1, CreatedAt: time.Now()},
		{ID: 2, Username: "user2", Email: "user2@example.com", PasswordHash: "hash2", IsEmailVerified: false, RoleID: 1, CreatedAt: time.Now()},
	}

	rows := sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_email_verified", "role_id", "created_at"})
	for _, u := range expectedUsers {
		rows.AddRow(u.ID, u.Username, u.Email, u.PasswordHash, u.IsEmailVerified, u.RoleID, u.CreatedAt)
	}

	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users`).
		WillReturnRows(rows)

	users, err := store.GetAll(context.Background())

	require.NoError(t, err)
	require.Len(t, users, 2)
	assert.Equal(t, expectedUsers[0].ID, users[0].ID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_Update_Success tests successful user update
func TestUsersStore_Update_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	user := &models.User{
		ID:              1,
		Username:        "updateduser",
		Email:           "updated@example.com",
		PasswordHash:    "$2a$10$newhash",
		IsEmailVerified: true,
	}

	mock.ExpectExec(`UPDATE users SET username = \$1, email = \$2, password_hash = \$3, is_email_verified = \$4 WHERE id = \$5`).
		WithArgs(user.Username, user.Email, user.PasswordHash, user.IsEmailVerified, user.ID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.Update(context.Background(), user)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_Delete_Success tests successful user deletion
func TestUsersStore_Delete_Success(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	id := "1"

	mock.ExpectExec(`DELETE FROM users WHERE id = \$1`).
		WithArgs(id).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := store.Delete(context.Background(), id)

	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_Create_AllFields tests that all user fields are saved correctly
func TestUsersStore_Create_AllFields(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	user := &models.User{
		Username:        "testuser",
		Email:           "test@example.com",
		PasswordHash:    "$2a$10$verylonghashedpasswordstring",
		IsEmailVerified: true, // Test with true
	}

	expectedID := int64(42)
	expectedCreatedAt := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(user.Username, user.Email, user.PasswordHash, user.IsEmailVerified).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(expectedID, expectedCreatedAt))

	err := store.Create(context.Background(), user)

	require.NoError(t, err)
	assert.Equal(t, expectedID, user.ID)
	assert.Equal(t, expectedCreatedAt, user.CreatedAt)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, "test@example.com", user.Email)
	assert.Equal(t, "$2a$10$verylonghashedpasswordstring", user.PasswordHash)
	assert.True(t, user.IsEmailVerified)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUsersStore_Create_ScanError(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	user := &models.User{
		Username:        "testuser",
		Email:           "test@example.com",
		PasswordHash:    "$2a$10$hash",
		IsEmailVerified: false,
	}

	// Return row missing created_at to force scan error
	mock.ExpectQuery(`INSERT INTO users`).
		WithArgs(user.Username, user.Email, user.PasswordHash, user.IsEmailVerified).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	err := store.Create(context.Background(), user)

	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// TestUsersStore_GetByEmail_AllFields tests that all user fields are retrieved correctly
func TestUsersStore_GetByEmail_AllFields(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	email := "test@example.com"
	expectedCreatedAt := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "is_email_verified", "role_id", "created_at"}).
			AddRow(1, "testuser", email, "$2a$10$hash", false, 1, expectedCreatedAt))

	user, err := store.GetByEmail(context.Background(), email)

	require.NoError(t, err)
	require.NotNil(t, user)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "testuser", user.Username)
	assert.Equal(t, email, user.Email)
	assert.Equal(t, "$2a$10$hash", user.PasswordHash)
	assert.False(t, user.IsEmailVerified)
	assert.Equal(t, 1, user.RoleID)
	assert.Equal(t, expectedCreatedAt, user.CreatedAt)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestUsersStore_GetByEmail_ScanError(t *testing.T) {
	db, mock, store := setupMockDB(t)
	defer db.Close()

	email := "test@example.com"

	// Return a row with missing columns to trigger scan error
	mock.ExpectQuery(`SELECT id, username, email, password_hash,
	is_email_verified, role_id, created_at FROM users WHERE email = \$1`).
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
			AddRow(1, "testuser"))

	user, err := store.GetByEmail(context.Background(), email)

	assert.Error(t, err)
	assert.Nil(t, user)
	assert.NoError(t, mock.ExpectationsWereMet())
}
