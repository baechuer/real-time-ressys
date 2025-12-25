package response

import (
	"errors"
	"net/http"

	"github.com/baechuer/real-time-ressys/services/event-service/internal/domain"
	zlog "github.com/rs/zerolog/log"
)

func Err(w http.ResponseWriter, err error) {
	requestID := ""

	if err == nil {
		Fail(w, http.StatusInternalServerError, "internal_error", "unknown error", nil, requestID)
		return
	}

	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal error"
	var meta map[string]string

	var ae *domain.AppError
	if errors.As(err, &ae) {
		status = statusFromCode(ae.Code)
		code = string(ae.Code)
		message = ae.Message
		meta = ae.Meta
		Fail(w, status, code, message, meta, requestID)
		return
	}

	// keep details in logs only
	zlog.Error().Err(err).Msg("unhandled error")
	Fail(w, http.StatusInternalServerError, code, message, nil, requestID)
}

func statusFromCode(code domain.ErrCode) int {
	switch code {
	case domain.CodeValidation:
		return http.StatusBadRequest
	case domain.CodeForbidden:
		return http.StatusForbidden
	case domain.CodeNotFound:
		return http.StatusNotFound
	case domain.CodeInvalidState:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
