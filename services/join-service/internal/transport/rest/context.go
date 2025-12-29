package rest

import (
	"context"

	"github.com/google/uuid"
)

type ctxKeyUserID struct{}
type ctxKeyRole struct{}
type ctxKeyTokenVer struct{}

type AuthContext struct {
	UserID uuid.UUID
	Role   string
	Ver    int64
}

func withAuth(ctx context.Context, a AuthContext) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID{}, a.UserID)
	ctx = context.WithValue(ctx, ctxKeyRole{}, a.Role)
	ctx = context.WithValue(ctx, ctxKeyTokenVer{}, a.Ver)
	return ctx
}

func GetAuth(ctx context.Context) (AuthContext, bool) {
	uid, ok := ctx.Value(ctxKeyUserID{}).(uuid.UUID)
	if !ok {
		return AuthContext{}, false
	}
	role, _ := ctx.Value(ctxKeyRole{}).(string)
	ver, _ := ctx.Value(ctxKeyTokenVer{}).(int64)

	return AuthContext{UserID: uid, Role: role, Ver: ver}, true
}
