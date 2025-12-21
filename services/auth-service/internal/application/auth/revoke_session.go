package auth

import (
	"context"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// RevokeUserSessions revokes all refresh sessions of a target user.
// Only admin should be able to call this (router/middleware enforces),
// but service still enforces core policy (defense in depth).
func (s *Service) RevokeUserSessions(ctx context.Context, actorID, actorRole, targetUserID string) error {
	const action = "admin.revoke_sessions"

	actorID = strings.TrimSpace(actorID)
	actorRole = strings.TrimSpace(actorRole)
	targetUserID = strings.TrimSpace(targetUserID)

	audit := func(result string, err error, extra map[string]string) {
		fields := map[string]string{
			"actor_id":   actorID,
			"actor_role": actorRole,
			"target_id":  targetUserID,
			"result":     result,
		}
		if err != nil {
			fields["error_code"] = domainCode(err)
		}
		for k, v := range extra {
			fields[k] = v
		}
		s.audit(action, fields)
	}

	if targetUserID == "" {
		err := domain.ErrMissingField("user_id")
		audit("error", err, nil)
		return err
	}

	// service-level enforcement (defense in depth)
	if !domain.IsValidRole(actorRole) {
		err := domain.ErrForbidden()
		audit("error", err, nil)
		return err
	}
	if domain.RoleRank(actorRole) < domain.RoleRank(string(domain.RoleAdmin)) {
		err := domain.ErrInsufficientRole(string(domain.RoleAdmin))
		audit("error", err, map[string]string{"required_role": string(domain.RoleAdmin)})
		return err
	}

	// hard rule: admin cannot affect self
	if actorID != "" && actorID == targetUserID {
		err := domain.ErrCannotAffectSelf()
		audit("error", err, nil)
		return err
	}

	// ensure user exists (so handler returns 404)
	if _, err := s.users.GetByID(ctx, targetUserID); err != nil {
		audit("error", err, nil)
		return err
	}

	// revoke all refresh tokens first, then audit
	if err := s.sessions.RevokeAll(ctx, targetUserID); err != nil {
		audit("error", err, nil)
		return err
	}

	audit("success", nil, nil)
	return nil
}

// SessionsRevoke revokes all refresh sessions for the authenticated user.
func (s *Service) SessionsRevoke(ctx context.Context, userID string) error {
	if userID == "" {
		return domain.ErrTokenMissing()
	}
	return s.sessions.RevokeAll(ctx, userID)
}
