package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestPasswordChange_TokenMissing(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordChange(context.Background(), "", "old", "newpasswordpassword")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrTokenMissing()))
}

func TestPasswordChange_WeakPassword(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordChange(context.Background(), "u1", "old", "short")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrWeakPassword("min length 12")))
}

func TestPasswordChange_OldPasswordMismatch_InvalidCredentials(t *testing.T) {
	t.Parallel()

	svc, users, hasher, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:old", Role: "user"}
	hasher.compareFn = func(hash, pw string) error { return errors.New("no") }

	err := svc.PasswordChange(context.Background(), "u1", "old", "newpasswordpassword")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInvalidCredentials()))
}

func TestPasswordChange_Success_UpdatesHash_RevokesAll(t *testing.T) {
	t.Parallel()

	svc, users, _, _, sessions, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:old", Role: "user"}

	err := svc.PasswordChange(context.Background(), "u1", "old", "newpasswordpassword")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(users.updatedPwd) != 1 {
		t.Fatalf("expected password hash updated once, got %v", users.updatedPwd)
	}
	if len(sessions.revokedAll) != 1 || sessions.revokedAll[0] != "u1" {
		t.Fatalf("expected revoke all for u1, got %v", sessions.revokedAll)
	}
}

func TestPasswordResetRequest_NonEnumerating_UserMissing_ReturnsNil(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, pub, _ := newSvcForTest(t)

	if err := svc.PasswordResetRequest(context.Background(), "missing@x.com"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(pub.resetEvts) != 0 {
		t.Fatalf("expected no publish when user missing")
	}
}

func TestPasswordResetRequest_Success_SavesToken_AndPublishes(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, ott, pub, _ := newSvcForTest(t)
	users.byEmail["e@x.com"] = domain.User{ID: "u1", Email: "e@x.com", Role: "user"}
	users.byID["u1"] = users.byEmail["e@x.com"]

	err := svc.PasswordResetRequest(context.Background(), "e@x.com")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(pub.resetEvts) != 1 {
		t.Fatalf("expected one reset publish, got %d", len(pub.resetEvts))
	}
	evt := pub.resetEvts[0]
	if evt.UserID != "u1" || evt.Email != "e@x.com" {
		t.Fatalf("unexpected evt: %+v", evt)
	}
	if !strings.HasPrefix(evt.URL, "https://fe/reset?token=") {
		t.Fatalf("unexpected url: %q", evt.URL)
	}
	// ensure token stored in OTT (peek the token portion)
	token := strings.TrimPrefix(evt.URL, "https://fe/reset?token=")
	if token == "" {
		t.Fatalf("expected token in url")
	}
	if _, err := ott.Peek(context.Background(), TokenPasswordReset, token); err != nil {
		t.Fatalf("expected token saved, peek err=%v", err)
	}
}

func TestPasswordResetValidate_EmptyToken(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordResetValidate(context.Background(), "")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrMissingField("token")))
}

func TestPasswordResetConfirm_Success_ConsumesAndUpdates_RevokesAll(t *testing.T) {
	t.Parallel()

	svc, users, _, _, sessions, ott, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:old", Role: "user"}
	_ = ott.Save(context.Background(), TokenPasswordReset, "tok", "u1", time.Minute)

	err := svc.PasswordResetConfirm(context.Background(), "tok", "newpasswordpassword")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(users.updatedPwd) != 1 {
		t.Fatalf("expected password updated, got %v", users.updatedPwd)
	}
	if len(sessions.revokedAll) != 1 || sessions.revokedAll[0] != "u1" {
		t.Fatalf("expected revoke all for u1, got %v", sessions.revokedAll)
	}
}
func TestPasswordResetConfirm_TokenNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordResetConfirm(context.Background(), "nope", "newpasswordpassword")
	requireErrCode(t, err, "reset_token_not_found")
}

func TestVerifyEmailRequest_EmptyEmail(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)
	err := svc.VerifyEmailRequest(context.Background(), "  ")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrMissingField("email")))
}

func TestVerifyEmailRequest_UserMissing_NonEnumerating_ReturnsNil(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, pub, _ := newSvcForTest(t)
	err := svc.VerifyEmailRequest(context.Background(), "missing@x.com")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(pub.verifyEvts) != 0 {
		t.Fatalf("expected no publish")
	}
}

func TestVerifyEmailRequest_Success_NormalizesEmail_PublishesAndSaves(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, ott, pub, _ := newSvcForTest(t)
	// repo expects lowercased trimmed
	users.byEmail["user@x.com"] = domain.User{ID: "u1", Email: "user@x.com", Role: "user"}
	users.byID["u1"] = users.byEmail["user@x.com"]

	err := svc.VerifyEmailRequest(context.Background(), "  USER@X.COM  ")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(pub.verifyEvts) != 1 {
		t.Fatalf("expected one publish")
	}
	evt := pub.verifyEvts[0]
	if !strings.HasPrefix(evt.URL, "https://fe/verify?token=") {
		t.Fatalf("unexpected url: %q", evt.URL)
	}
	token := strings.TrimPrefix(evt.URL, "https://fe/verify?token=")
	if _, err := ott.Peek(context.Background(), TokenVerifyEmail, token); err != nil {
		t.Fatalf("expected token saved, peek err=%v", err)
	}
}

func TestVerifyEmailConfirm_EmptyToken(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)
	err := svc.VerifyEmailConfirm(context.Background(), " ")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrMissingField("token")))
}

func TestVerifyEmailConfirm_Success_ConsumesAndSetsVerified(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, ott, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", EmailVerified: false}
	_ = ott.Save(context.Background(), TokenVerifyEmail, "tok", "u1", time.Minute)

	err := svc.VerifyEmailConfirm(context.Background(), "tok")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	u := users.byID["u1"]
	if !u.EmailVerified {
		t.Fatalf("expected email verified true")
	}
}
func TestVerifyEmailConfirm_TokenNotFound(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.VerifyEmailConfirm(context.Background(), "nope")
	requireErrCode(t, err, "verify_token_not_found")
}
func TestPasswordResetConfirm_EmptyNewPassword_MissingField(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordResetConfirm(context.Background(), "tok", "")
	requireErrCode(t, err, "missing_field")
}

func TestPasswordResetConfirm_WeakPassword(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	err := svc.PasswordResetConfirm(context.Background(), "tok", "short")
	requireErrCode(t, err, "weak_password")
}

func TestPasswordResetConfirm_HashFails_ReturnsHashFailed(t *testing.T) {
	t.Parallel()

	svc, users, hasher, _, sessions, ott, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:old", Role: "user"}
	_ = ott.Save(context.Background(), TokenPasswordReset, "tok", "u1", time.Minute)

	hasher.hashFn = func(pw string) (string, error) { return "", errors.New("hash fail") }

	err := svc.PasswordResetConfirm(context.Background(), "tok", "newpasswordpassword")
	requireErrCode(t, err, "hash_failed")

	// should NOT revoke all if hash fails
	if len(sessions.revokedAll) != 0 {
		t.Fatalf("expected no revokeAll on hash failure")
	}
}

func TestPasswordResetConfirm_UpdatePasswordFails_ReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, sessions, ott, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:old", Role: "user"}
	_ = ott.Save(context.Background(), TokenPasswordReset, "tok", "u1", time.Minute)

	users.updatePwdErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.PasswordResetConfirm(context.Background(), "tok", "newpasswordpassword")
	requireErrCode(t, err, "db_unavailable")

	// should NOT revoke all if update fails (your code revokes after update)
	if len(sessions.revokedAll) != 0 {
		t.Fatalf("expected no revokeAll when update password fails")
	}
}

func TestVerifyEmailConfirm_SetVerifiedFails_ReturnsUnderlyingError(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, ott, _, _ := newSvcForTest(t)

	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", EmailVerified: false}
	_ = ott.Save(context.Background(), TokenVerifyEmail, "tok", "u1", time.Minute)

	users.setVerifiedErr = domain.ErrDBUnavailable(errors.New("db down"))

	err := svc.VerifyEmailConfirm(context.Background(), "tok")
	requireErrCode(t, err, "db_unavailable")
}
