package response

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func Err(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal error"
	var meta map[string]string

	var ae *domain.AppError
	if errors.As(err, &ae) {
		// map domain error -> http status
		switch ae.Code {
		case domain.CodeValidation:
			status = http.StatusBadRequest
			code = "validation_error"
		case domain.CodeForbidden:
			status = http.StatusForbidden
			code = "forbidden"
		case domain.CodeNotFound:
			status = http.StatusNotFound
			code = "not_found"
		case domain.CodeInvalidState:
			status = http.StatusConflict
			code = "invalid_state"
		default:
			status = http.StatusBadRequest
			code = "validation_error"
		}

		message = ae.Message
		// 如果你有 meta（比如 ErrValidationMeta），这里接上：
		// meta = ae.Meta
	}

	reqID := chimw.GetReqID(r.Context())

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorBody{
		Error: ErrorPayload{
			Code:      code,
			Message:   message,
			Meta:      meta,
			RequestID: reqID,
		},
	})
}
