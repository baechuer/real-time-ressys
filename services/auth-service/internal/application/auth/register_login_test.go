package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestRegister_Empty_ReturnsInvalidField(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	_, err := svc.Register(context.Background(), "", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInvalidField("email/password", "empty")))
}

func TestRegister_HashFail_ReturnsHashFailed(t *testing.T) {
	t.Parallel()

	svc, _, hasher, _, _, _, _, _ := newSvcForTest(t)
	hasher.hashFn = func(pw string) (string, error) { return "", errors.New("boom") }

	_, err := svc.Register(context.Background(), "a@b.com", "pw")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrHashFailed(errors.New("x")))) // code stable; cause differs
}

func TestRegister_Success_IssuesTokens_AndPersistsUser(t *testing.T) {
	t.Parallel()

	svc, users, _, _, sessions, _, _, _ := newSvcForTest(t)

	res, err := svc.Register(context.Background(), "a@b.com", "pw")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if res.User.ID == "" {
		t.Fatalf("expected user ID set")
	}
	if res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Fatalf("expected tokens, got %+v", res.Tokens)
	}
	if _, ok := users.byID[res.User.ID]; !ok {
		t.Fatalf("expected user stored by id")
	}
	if _, ok := sessions.byToken[res.Tokens.RefreshToken]; !ok {
		t.Fatalf("expected refresh stored")
	}
}

func TestLogin_EmptyFields_InvalidCredentials(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	_, err := svc.Login(context.Background(), "", "")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInvalidCredentials()))
}

func TestLogin_UserNotFound_NonEnumerating_InvalidCredentials(t *testing.T) {
	t.Parallel()

	svc, _, _, _, _, _, _, _ := newSvcForTest(t)

	_, err := svc.Login(context.Background(), "missing@x.com", "pw")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInvalidCredentials()))
}

func TestLogin_BadPassword_InvalidCredentials(t *testing.T) {
	t.Parallel()

	svc, users, hasher, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:pw", Role: "user"}
	users.byEmail["e@x.com"] = users.byID["u1"]

	hasher.compareFn = func(hash, pw string) error { return errors.New("nope") }

	_, err := svc.Login(context.Background(), "e@x.com", "pw")
	if err == nil {
		t.Fatalf("expected error")
	}
	requireDomainCode(t, err, domainCode(domain.ErrInvalidCredentials()))
}

func TestLogin_Success_IssuesTokens(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	u := domain.User{ID: "u1", Email: "e@x.com", PasswordHash: "hash:pw", Role: "user"}
	users.byID[u.ID] = u
	users.byEmail[u.Email] = u

	res, err := svc.Login(context.Background(), "  e@x.com  ", "pw")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if res.User.ID != "u1" {
		t.Fatalf("expected user u1, got %+v", res.User)
	}
	if res.Tokens.AccessToken == "" || res.Tokens.RefreshToken == "" {
		t.Fatalf("expected tokens, got %+v", res.Tokens)
	}
}
func TestRegister_CreateErr_EmailAlreadyExists(t *testing.T) {
	t.Parallel()

	newSvcForTest(t)
	// make repo create fail as conflict
	_, users, _, _, _, _, _, _ := newSvcForTest(t)
	_ = users // keep compiler happy if you already have users returned in this file
	// Better: just rebuild properly:
	svc2, users2, _, _, _, _, _, _ := newSvcForTest(t)
	users2.createErr = domain.ErrEmailAlreadyExists()

	_, err := svc2.Register(context.Background(), "a@b.com", "passwordpassword")
	requireErrCode(t, err, "email_already_exists")
}
