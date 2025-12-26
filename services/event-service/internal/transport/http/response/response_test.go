package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestErr(t *testing.T) {
	t.Run("maps_domain_error_to_correct_status", func(t *testing.T) {
		tests := []struct {
			name       string
			err        error
			wantStatus int
			wantCode   string
		}{
			{
				name:       "not_found",
				err:        domain.ErrNotFound("event missing"),
				wantStatus: http.StatusNotFound,
				wantCode:   "not_found",
			},
			{
				name:       "validation",
				err:        domain.ErrValidation("invalid title"),
				wantStatus: http.StatusBadRequest,
				wantCode:   "validation_error",
			},
			{
				name:       "forbidden",
				err:        domain.ErrForbidden("no access"),
				wantStatus: http.StatusForbidden,
				wantCode:   "forbidden",
			},
			{
				name:       "generic_error",
				err:        errors.New("db crash"),
				wantStatus: http.StatusInternalServerError,
				wantCode:   "internal_error",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rr := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
				Err(rr, req, tt.err)

				assert.Equal(t, tt.wantStatus, rr.Code)

				var body ErrorBody
				err := json.Unmarshal(rr.Body.Bytes(), &body)
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCode, body.Error.Code)
			})
		}
	})
}

func TestData(t *testing.T) {
	t.Run("wraps_payload_in_data_envelope", func(t *testing.T) {
		rr := httptest.NewRecorder()
		payload := map[string]string{"id": "123"}

		Data(rr, http.StatusOK, payload)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "application/json; charset=utf-8", rr.Header().Get("Content-Type"))

		var env Envelope
		err := json.Unmarshal(rr.Body.Bytes(), &env)
		assert.NoError(t, err)

		// Unmarshal Data into a map to check content
		dataMap := env.Data.(map[string]any)
		assert.Equal(t, "123", dataMap["id"])
	})
}
