package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/baechuer/real-time-ressys/services/auth-service/internal/domain"
)

func TestGetUserByID_Passthrough(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", Role: "user"}

	u, err := svc.GetUserByID(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if u.ID != "u1" {
		t.Fatalf("expected u1, got %+v", u)
	}
}

func TestGetMyStatus_ReturnsProjectedFields(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u1"] = domain.User{ID: "u1", Email: "e@x.com", Role: "user", Locked: true, EmailVerified: false}

	st, err := svc.GetMyStatus(context.Background(), "u1")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if st.UserID != "u1" || st.Role != "user" || !st.Locked || st.EmailVerified {
		t.Fatalf("unexpected status: %+v", st)
	}
}

func TestGetUserStatus_ReturnsProjectedFields(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.byID["u2"] = domain.User{ID: "u2", Email: "e2@x.com", Role: "admin", Locked: false, EmailVerified: true}

	st, err := svc.GetUserStatus(context.Background(), "u2")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if st.UserID != "u2" || st.Role != "admin" || st.Locked || !st.EmailVerified {
		t.Fatalf("unexpected status: %+v", st)
	}
}
func TestGetMyStatus_UserRepoError_Passthrough(t *testing.T) {
	t.Parallel()

	svc, users, _, _, _, _, _, _ := newSvcForTest(t)
	users.getByIDErr = domain.ErrDBUnavailable(errors.New("db down"))

	_, err := svc.GetMyStatus(context.Background(), "u1")
	requireErrCode(t, err, "db_unavailable")
}
