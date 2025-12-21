package auth

import (
	"context"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func (s *Service) SetUserRole(
	ctx context.Context,
	actorID, actorRole, targetUserID, newRole string,
) error {
	const action = "admin.set_user_role"

	actorID = strings.TrimSpace(actorID)
	actorRole = strings.TrimSpace(actorRole)
	targetUserID = strings.TrimSpace(targetUserID)
	newRole = strings.TrimSpace(newRole)

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

	// --- input validation ---
	if targetUserID == "" {
		err := domain.ErrMissingField("user_id")
		audit("error", err, nil)
		return err
	}
	if newRole == "" {
		err := domain.ErrMissingField("role")
		audit("error", err, nil)
		return err
	}
	if !domain.IsValidRole(newRole) {
		err := domain.ErrInvalidField("role", "invalid role")
		audit("error", err, nil)
		return err
	}

	// --- RBAC: admin only (defense in depth) ---
	if !domain.IsValidRole(actorRole) {
		err := domain.ErrForbidden()
		audit("error", err, nil)
		return err
	}
	if domain.RoleRank(actorRole) < domain.RoleRank(string(domain.RoleAdmin)) {
		err := domain.ErrInsufficientRole(string(domain.RoleAdmin))
		audit("error", err, map[string]string{
			"required_role": string(domain.RoleAdmin),
		})
		return err
	}

	// --- hard rule: cannot modify self ---
	if actorID != "" && actorID == targetUserID {
		err := domain.ErrCannotAffectSelf()
		audit("error", err, nil)
		return err
	}

	// --- ensure target exists & get current role ---
	target, err := s.users.GetByID(ctx, targetUserID)
	if err != nil {
		audit("error", err, nil)
		return err
	}

	// --- protect last admin ---
	if target.Role == string(domain.RoleAdmin) &&
		newRole != string(domain.RoleAdmin) {

		cnt, err := s.users.CountByRole(ctx, string(domain.RoleAdmin))
		if err != nil {
			audit("error", err, nil)
			return err
		}
		if cnt <= 1 {
			err := domain.ErrLastAdminProtected()
			audit("error", err, nil)
			return err
		}
	}

	// --- apply change ---
	if err := s.users.SetRole(ctx, targetUserID, newRole); err != nil {
		audit("error", err, nil)
		return err
	}

	audit("success", nil, map[string]string{
		"new_role": newRole,
	})
	return nil
}
