package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestBanUser_TargetMissing(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, audits := newSvcForTest(t)

	err := svc.BanUser(context.Background(), "a1", "admin", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrMissingField("user_id")))

	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "result", "error")
}

func TestBanUser_InvalidActorRole_Forbidden(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, audits := newSvcForTest(t)

	err := svc.BanUser(context.Background(), "a1", "???", "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrForbidden()))

	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "result", "error")
}

func TestBanUser_InsufficientRole(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, audits := newSvcForTest(t)

	err := svc.BanUser(context.Background(), "a1", "user", "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInsufficientRole(string(domain.RoleModerator))))

	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "required_role", string(domain.RoleModerator))
}

func TestBanUser_CannotModerateSelf(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)
	err := svc.BanUser(context.Background(), "u1", "admin", "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrCannotModerateSelf()))
}

func TestBanUser_ModCannotBanAdmin(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["target"] = domain.User{ID: "target", Email: "t@x.com", Role: string(domain.RoleAdmin)}
	users.byEmail["t@x.com"] = users.byID["target"]

	err := svc.BanUser(context.Background(), "mod1", string(domain.RoleModerator), "target")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrCannotModerateAdmin()))
}

func TestBanUser_Success_LocksUser_AuditsSuccess(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, audits := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}
	users.byEmail["u@x.com"] = users.byID["u1"]

	err := svc.BanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if got := users.byID["u1"]; !got.Locked {
		t.Fatalf("expected locked=true")
	}
	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "result", "success")
}

func TestUnbanUser_InsufficientRole(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.UnbanUser(context.Background(), "u1", "user", "target")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInsufficientRole(string(domain.RoleModerator))))
}

func TestUnbanUser_ModCannotUnbanAdmin(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["target"] = domain.User{ID: "target", Email: "t@x.com", Role: string(domain.RoleAdmin)}
	users.byEmail["t@x.com"] = users.byID["target"]

	err := svc.UnbanUser(context.Background(), "mod1", string(domain.RoleModerator), "target")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrCannotModerateAdmin()))
}

func TestUnbanUser_Success_UnlocksUser(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user", Locked: true}
	users.byEmail["u@x.com"] = users.byID["u1"]

	err := svc.UnbanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if users.byID["u1"].Locked {
		t.Fatalf("expected locked=false")
	}
}

func TestSetUserRole_ProtectLastAdmin(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["admin"] = domain.User{ID: "admin", Email: "a@x.com", Role: string(domain.RoleAdmin)}
	users.byEmail["a@x.com"] = users.byID["admin"]

	err := svc.SetUserRole(context.Background(), "admin2", string(domain.RoleAdmin), "admin", "user")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrLastAdminProtected()))
}

func TestSetUserRole_Success(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	// two admins to avoid last-admin protection
	users.byID["a1"] = domain.User{ID: "a1", Email: "a1@x.com", Role: string(domain.RoleAdmin)}
	users.byID["a2"] = domain.User{ID: "a2", Email: "a2@x.com", Role: string(domain.RoleAdmin)}
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "u1", string(domain.RoleModerator))
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if users.byID["u1"].Role != string(domain.RoleModerator) {
		t.Fatalf("expected role=%s, got %s", string(domain.RoleModerator), users.byID["u1"].Role)
	}
}

func TestRevokeUserSessions_AdminOnly_AndCannotSelf(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	// not admin -> insufficient role
	err := svc.RevokeUserSessions(context.Background(), "a1", "user", "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInsufficientRole(string(domain.RoleAdmin))))

	// admin self -> cannot affect self
	err = svc.RevokeUserSessions(context.Background(), "u1", string(domain.RoleAdmin), "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrCannotAffectSelf()))
}

func TestRevokeUserSessions_Success_RevokesAll(t *testing.T) {
	t.Parallel()

	svc, users, _, _, sessions, _, _, _ := newSvcForTest(t)

	// make target exist
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	// seed some sessions
	sessions.byToken["rft:u1"] = "u1"
	sessions.byToken["rft2:u1"] = "u1"

	err := svc.RevokeUserSessions(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(sessions.revokedAll) != 1 || sessions.revokedAll[0] != "u1" {
		t.Fatalf("expected revoke all u1, got %v", sessions.revokedAll)
	}
	if len(sessions.byToken) != 0 {
		t.Fatalf("expected all tokens removed, got %v", sessions.byToken)
	}

	// also test infra error path quickly
	sessions.revokeAllErr = errors.New("redis down")
	err = svc.RevokeUserSessions(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	if err == nil {
		t.Fatalf("expected error")
	}
}
func TestRevokeUserSessions_TargetNotFound_ReturnsUserNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.RevokeUserSessions(context.Background(), "admin1", string(domain.RoleAdmin), "missing")
	requireErrCode(t, err, "user_not_found")
}
func TestBanUser_TargetNotFound_ReturnsUserNotFound_AuditsError(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, audits := newSvcForTest(t)

	err := svc.BanUser(context.Background(), "admin1", string(domain.RoleAdmin), "missing")
	requireErrCode(t, err, "user_not_found")

	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "result", "error")
}

func TestBanUser_TargetRoleInvalid_ReturnsInternalError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "not_a_role"}
	users.byEmail["u@x.com"] = users.byID["u1"]

	err := svc.BanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	requireErrCode(t, err, "internal_error")
}

func TestBanUser_LockFails_ReturnsUnderlyingError_AuditsError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, audits := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}
	users.byEmail["u@x.com"] = users.byID["u1"]

	users.lockErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.BanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	requireErrCode(t, err, "db_unavailable")

	e := requireAuditAction(t, audits, "mod.ban_user")
	requireAuditField(t, e, "result", "error")
}

func TestUnbanUser_TargetRoleInvalid_ReturnsInternalError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "???", Locked: true}
	users.byEmail["u@x.com"] = users.byID["u1"]

	err := svc.UnbanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	requireErrCode(t, err, "internal_error")
}

func TestUnbanUser_UnlockFails_ReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user", Locked: true}
	users.byEmail["u@x.com"] = users.byID["u1"]

	users.unlockErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.UnbanUser(context.Background(), "admin1", string(domain.RoleAdmin), "u1")
	requireErrCode(t, err, "db_unavailable")
}
func TestSetUserRole_TargetMissingField(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "", "user")
	requireErrCode(t, err, "missing_field")
}

func TestSetUserRole_NewRoleMissingField(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "u1", "")
	requireErrCode(t, err, "missing_field")
}

func TestSetUserRole_NewRoleInvalidField(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	// target must exist for later steps; but this should fail earlier at role validation
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "u1", "not_a_role")
	requireErrCode(t, err, "invalid_field")
}

func TestSetUserRole_InvalidActorRole_Forbidden(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	err := svc.SetUserRole(context.Background(), "a1", "???", "u1", "user")
	requireErrCode(t, err, "forbidden")
}

func TestSetUserRole_InsufficientRole(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	err := svc.SetUserRole(context.Background(), "u2", "user", "u1", "moderator")
	requireErrCode(t, err, "insufficient_role")
}

func TestSetUserRole_CannotAffectSelf(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "admin"}

	err := svc.SetUserRole(context.Background(), "u1", string(domain.RoleAdmin), "u1", "user")
	requireErrCode(t, err, "cannot_affect_self")
}

func TestSetUserRole_TargetNotFound_ReturnsUserNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "missing", "user")
	requireErrCode(t, err, "user_not_found")
}

func TestSetUserRole_CountByRoleFails_ReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)

	// two admins, but we’ll force count error anyway
	users.byID["a1"] = domain.User{ID: "a1", Email: "a1@x.com", Role: string(domain.RoleAdmin)}
	users.byID["a2"] = domain.User{ID: "a2", Email: "a2@x.com", Role: string(domain.RoleAdmin)}
	users.byID["target"] = domain.User{ID: "target", Email: "t@x.com", Role: string(domain.RoleAdmin)}

	users.countByRoleErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "target", "user")
	requireErrCode(t, err, "db_unavailable")
}

func TestSetUserRole_SetRoleFails_ReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)

	// ensure not last admin protection triggers: have two admins
	users.byID["a1"] = domain.User{ID: "a1", Email: "a1@x.com", Role: string(domain.RoleAdmin)}
	users.byID["a2"] = domain.User{ID: "a2", Email: "a2@x.com", Role: string(domain.RoleAdmin)}
	users.byID["u1"] = domain.User{ID: "u1", Email: "u@x.com", Role: "user"}

	users.setRoleErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.SetUserRole(context.Background(), "a1", string(domain.RoleAdmin), "u1", "moderator")
	requireErrCode(t, err, "db_unavailable")
}
func TestRevokeUserSessions_TargetMissingField(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.RevokeUserSessions(context.Background(), "a1", string(domain.RoleAdmin), "")
	requireErrCode(t, err, "missing_field")
}
