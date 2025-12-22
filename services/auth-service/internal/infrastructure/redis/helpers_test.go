package redis

import (
	"errors"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func isMissingField(err error, field string) bool {
	var de *domain.Error
	if errors.As(err, &de) {
		return de.Code == "missing_field" && de.Meta != nil && de.Meta["field"] == field
	}
	return false
}
