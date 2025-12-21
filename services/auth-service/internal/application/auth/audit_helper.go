package auth

import "github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"

func domainCode(err error) string {
	if err == nil {
		return ""
	}
	if de, ok := err.(*domain.Error); ok {
		return de.Code
	}
	return "non_domain_error"
}
