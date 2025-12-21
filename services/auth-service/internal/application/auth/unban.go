package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

// UnbanUser unlocks (unbans) a target user account.
// Hard rules enforced here:
// - Nobody can unban themselves
// - Moderator cannot unban admin
// - Requires at least moderator
func (s *Service) UnbanUser(
	ctx context.Context,
	actorID, actorRole, targetUserID string,
) error {
	const action = "admin.unban_user"

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

	// --- input validation ---
	if targetUserID == "" {
		err := domain.ErrMissingField("user_id")
		audit("error", err, nil)
		return err
	}

	// --- RBAC: moderator+ only (defense in depth) ---
	if !domain.IsValidRole(actorRole) {
		err := domain.ErrForbidden()
		audit("error", err, nil)
		return err
	}
	if domain.RoleRank(actorRole) < domain.RoleRank(string(domain.RoleModerator)) {
		err := domain.ErrInsufficientRole(string(domain.RoleModerator))
		audit("error", err, map[string]string{
			"required_role": string(domain.RoleModerator),
		})
		return err
	}

	// --- hard rule: cannot moderate self ---
	if actorID != "" && actorID == targetUserID {
		err := domain.ErrCannotModerateSelf()
		audit("error", err, nil)
		return err
	}

	// --- load target user ---
	target, err := s.users.GetByID(ctx, targetUserID)
	if err != nil {
		audit("error", err, nil)
		return err
	}

	// --- integrity check: stored role must be valid ---
	if !domain.IsValidRole(target.Role) {
		err := domain.ErrInternal(
			fmt.Errorf("invalid stored role for user %s: %q", targetUserID, target.Role),
		)
		audit("error", err, nil)
		return err
	}

	// --- rule: moderator cannot unban admin ---
	if actorRole == string(domain.RoleModerator) &&
		target.Role == string(domain.RoleAdmin) {

		err := domain.ErrCannotModerateAdmin()
		audit("error", err, nil)
		return err
	}

	// --- apply change ---
	if err := s.users.UnlockUser(ctx, targetUserID); err != nil {
		audit("error", err, nil)
		return err
	}

	audit("success", nil, nil)
	return nil
}
