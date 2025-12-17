package docs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIHandler_ReturnsJSON(t *testing.T) {
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	rec := httptest.NewRecorder()

	OpenAPIHandler(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
}

func TestOpenAPIHandler_ValidSpec(t *testing.T) {
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	rec := httptest.NewRecorder()

	OpenAPIHandler(rec, req)

	var spec OpenAPISpec
	err := json.NewDecoder(rec.Body).Decode(&spec)
	require.NoError(t, err)

	assert.Equal(t, "3.0.3", spec.OpenAPI)
	assert.Equal(t, "Auth Service API", spec.Info.Title)
	assert.Equal(t, "1.0.0", spec.Info.Version)
}

func TestOpenAPIHandler_ContainsEndpoints(t *testing.T) {
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	rec := httptest.NewRecorder()

	OpenAPIHandler(rec, req)

	var spec OpenAPISpec
	err := json.NewDecoder(rec.Body).Decode(&spec)
	require.NoError(t, err)

	// Check that key endpoints are present
	assert.Contains(t, spec.Paths, "/auth/v1/health")
	assert.Contains(t, spec.Paths, "/auth/v1/register")
	assert.Contains(t, spec.Paths, "/auth/v1/login")
	assert.Contains(t, spec.Paths, "/auth/v1/me")
}

func TestOpenAPIHandler_ContainsServers(t *testing.T) {
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	rec := httptest.NewRecorder()

	OpenAPIHandler(rec, req)

	var spec OpenAPISpec
	err := json.NewDecoder(rec.Body).Decode(&spec)
	require.NoError(t, err)

	assert.NotEmpty(t, spec.Servers)
	assert.Equal(t, "http://localhost:8080", spec.Servers[0].URL)
}
