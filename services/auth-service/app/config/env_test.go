package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

/*
Env config test cases:
1) GetString existing var
2) GetString missing var → fallback
3) GetString empty value preserved
4) GetString special characters
5) GetInt valid integer
6) GetInt missing var → fallback
7) GetInt invalid integer → fallback
8) GetInt empty value → fallback
9) GetInt negative integer
10) GetInt zero value
11) GetInt large integer
12) Load without .env (no panic)
13) Load with .env present or absent (no panic)
*/

// TestGetString_ExistingVar tests GetString with existing environment variable
func TestGetString_ExistingVar(t *testing.T) {
	key := "TEST_GETSTRING_EXISTING"
	expectedValue := "test_value"

	os.Setenv(key, expectedValue)
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetString(key, "fallback")
	assert.Equal(t, expectedValue, result, "Should return the environment variable value")
}

// TestGetString_NonExistentVar tests GetString with non-existent environment variable
func TestGetString_NonExistentVar(t *testing.T) {
	key := "TEST_GETSTRING_NONEXISTENT"
	fallback := "fallback_value"

	// Ensure the key doesn't exist
	os.Unsetenv(key)
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetString(key, fallback)
	assert.Equal(t, fallback, result, "Should return fallback when env var doesn't exist")
}

// TestGetString_EmptyValue tests GetString with empty environment variable value
func TestGetString_EmptyValue(t *testing.T) {
	key := "TEST_GETSTRING_EMPTY"
	fallback := "fallback_value"

	os.Setenv(key, "")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetString(key, fallback)
	// Important: Empty string is a valid value, so it should return empty, not fallback
	assert.Equal(t, "", result, "Should return empty string when env var exists but is empty")
	assert.NotEqual(t, fallback, result, "Should not return fallback for empty value")
}

// TestGetString_SpecialCharacters tests GetString with special characters in value
func TestGetString_SpecialCharacters(t *testing.T) {
	key := "TEST_GETSTRING_SPECIAL"
	expectedValue := "value with spaces and @#$%^&*()"

	os.Setenv(key, expectedValue)
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetString(key, "fallback")
	assert.Equal(t, expectedValue, result, "Should handle special characters correctly")
}

// TestGetInt_ValidInteger tests GetInt with valid integer
func TestGetInt_ValidInteger(t *testing.T) {
	key := "TEST_GETINT_VALID"
	expectedValue := 42

	os.Setenv(key, "42")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, 0)
	assert.Equal(t, expectedValue, result, "Should return parsed integer value")
}

// TestGetInt_NonExistentVar tests GetInt with non-existent environment variable
func TestGetInt_NonExistentVar(t *testing.T) {
	key := "TEST_GETINT_NONEXISTENT"
	fallback := 99

	// Ensure the key doesn't exist
	os.Unsetenv(key)
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, fallback)
	assert.Equal(t, fallback, result, "Should return fallback when env var doesn't exist")
}

// TestGetInt_InvalidInteger tests GetInt with invalid integer value
func TestGetInt_InvalidInteger(t *testing.T) {
	key := "TEST_GETINT_INVALID"
	fallback := 99

	os.Setenv(key, "not_a_number")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, fallback)
	assert.Equal(t, fallback, result, "Should return fallback when value cannot be parsed as integer")
}

// TestGetInt_EmptyValue tests GetInt with empty environment variable value
func TestGetInt_EmptyValue(t *testing.T) {
	key := "TEST_GETINT_EMPTY"
	fallback := 99

	os.Setenv(key, "")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, fallback)
	assert.Equal(t, fallback, result, "Should return fallback when env var is empty")
}

// TestGetInt_NegativeInteger tests GetInt with negative integer
func TestGetInt_NegativeInteger(t *testing.T) {
	key := "TEST_GETINT_NEGATIVE"
	expectedValue := -10

	os.Setenv(key, "-10")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, 0)
	assert.Equal(t, expectedValue, result, "Should handle negative integers correctly")
}

// TestGetInt_ZeroValue tests GetInt with zero value
func TestGetInt_ZeroValue(t *testing.T) {
	key := "TEST_GETINT_ZERO"
	expectedValue := 0

	os.Setenv(key, "0")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, 99)
	assert.Equal(t, expectedValue, result, "Should return zero when env var is '0'")
}

// TestGetInt_LargeInteger tests GetInt with large integer value
func TestGetInt_LargeInteger(t *testing.T) {
	key := "TEST_GETINT_LARGE"
	expectedValue := 2147483647 // Max int32

	os.Setenv(key, "2147483647")
	t.Cleanup(func() {
		os.Unsetenv(key)
	})

	result := GetInt(key, 0)
	assert.Equal(t, expectedValue, result, "Should handle large integers correctly")
}

// TestLoad_WithoutEnvFile tests Load when .env file doesn't exist
// Load() should not panic or error, just log a warning
func TestLoad_WithoutEnvFile(t *testing.T) {
	// This test verifies that Load() doesn't crash when .env doesn't exist
	// The function should handle the error gracefully and just log a warning
	// We can't easily test the log output, but we can verify it doesn't panic

	// Ensure .env doesn't exist (or rename it temporarily)
	// Note: This test assumes .env might not exist, which is fine

	// Should not panic
	assert.NotPanics(t, func() {
		Load()
	}, "Load() should not panic when .env file doesn't exist")

	// Can be called multiple times safely
	assert.NotPanics(t, func() {
		Load()
		Load()
	}, "Load() should be safe to call multiple times")
}

// TestLoad_WithEnvFile tests Load when .env file exists
// Note: This test requires a .env file to exist, so it's optional
func TestLoad_WithEnvFile(t *testing.T) {
	// This test verifies Load() works when .env exists
	// Since we can't easily create a temporary .env file in the project root
	// without affecting other tests, we'll just verify it doesn't panic

	// If .env exists, Load() should work without error
	// If .env doesn't exist, it should still not panic (just log warning)
	assert.NotPanics(t, func() {
		Load()
	}, "Load() should not panic regardless of .env file existence")
}
