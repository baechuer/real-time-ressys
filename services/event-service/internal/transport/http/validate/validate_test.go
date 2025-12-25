package validate

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsUUID(t *testing.T) {
	t.Run("valid_uuid", func(t *testing.T) {
		assert.True(t, IsUUID("550e8400-e29b-41d4-a716-446655440000"))
	})

	t.Run("invalid_uuid_string", func(t *testing.T) {
		assert.False(t, IsUUID("not-a-uuid"))
	})

	t.Run("empty_string", func(t *testing.T) {
		assert.False(t, IsUUID(""))
	})
}

func TestDecodeJSON(t *testing.T) {
	type testStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	t.Run("valid_json_decoding", func(t *testing.T) {
		body := `{"name": "Sydney", "age": 200}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))

		var dst testStruct
		err := DecodeJSON(req, &dst)

		assert.NoError(t, err)
		assert.Equal(t, "Sydney", dst.Name)
		assert.Equal(t, 200, dst.Age)
	})

	t.Run("fail_on_unknown_fields", func(t *testing.T) {
		// Because we use DisallowUnknownFields()
		body := `{"name": "Syd", "unknown_field": true}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))

		var dst testStruct
		err := DecodeJSON(req, &dst)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown field")
	})

	t.Run("fail_on_malformed_json", func(t *testing.T) {
		body := `{"name": "Syd",` // Missing closing brace
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))

		var dst testStruct
		err := DecodeJSON(req, &dst)

		assert.Error(t, err)
	})
}
