package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	// Load should not fail even if .env doesn't exist
	err := Load()
	assert.NoError(t, err)
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		setValue     string
		defaultValue string
		expected     string
	}{
		{
			name:         "value exists",
			key:          "TEST_STRING_KEY",
			setValue:     "test-value",
			defaultValue: "default",
			expected:     "test-value",
		},
		{
			name:         "value not set",
			key:          "TEST_STRING_KEY_NOT_SET",
			setValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "empty value",
			key:          "TEST_STRING_KEY_EMPTY",
			setValue:     "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up after test
			defer os.Unsetenv(tt.key)

			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
			}

			result := GetString(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		setValue     string
		defaultValue int
		expected     int
	}{
		{
			name:         "valid integer",
			key:          "TEST_INT_KEY",
			setValue:     "42",
			defaultValue: 10,
			expected:     42,
		},
		{
			name:         "value not set",
			key:          "TEST_INT_KEY_NOT_SET",
			setValue:     "",
			defaultValue: 10,
			expected:     10,
		},
		{
			name:         "invalid integer",
			key:          "TEST_INT_KEY_INVALID",
			setValue:     "not-a-number",
			defaultValue: 10,
			expected:     10, // Should return default on parse error
		},
		{
			name:         "negative integer",
			key:          "TEST_INT_KEY_NEGATIVE",
			setValue:     "-5",
			defaultValue: 10,
			expected:     -5,
		},
		{
			name:         "zero",
			key:          "TEST_INT_KEY_ZERO",
			setValue:     "0",
			defaultValue: 10,
			expected:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up after test
			defer os.Unsetenv(tt.key)

			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
			}

			result := GetInt(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetBool(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		setValue     string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "true value",
			key:          "TEST_BOOL_KEY_TRUE",
			setValue:     "true",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "false value",
			key:          "TEST_BOOL_KEY_FALSE",
			setValue:     "false",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "value not set",
			key:          "TEST_BOOL_KEY_NOT_SET",
			setValue:     "",
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "invalid bool",
			key:          "TEST_BOOL_KEY_INVALID",
			setValue:     "not-a-bool",
			defaultValue: true,
			expected:     true, // Should return default on parse error
		},
		{
			name:         "1 as true",
			key:          "TEST_BOOL_KEY_ONE",
			setValue:     "1",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "0 as false",
			key:          "TEST_BOOL_KEY_ZERO",
			setValue:     "0",
			defaultValue: true,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up after test
			defer os.Unsetenv(tt.key)

			if tt.setValue != "" {
				os.Setenv(tt.key, tt.setValue)
			}

			result := GetBool(tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetString_WithEnvFile(t *testing.T) {
	// Test that Load() can handle missing .env file
	err := Load()
	require.NoError(t, err)

	// Should still work with environment variables
	result := GetString("NON_EXISTENT_KEY", "default")
	assert.Equal(t, "default", result)
}

