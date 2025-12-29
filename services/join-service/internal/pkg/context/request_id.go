package context

import "context"

type requestIDKey struct{}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

func GetRequestID(ctx context.Context) string {
	v := ctx.Value(requestIDKey{})
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
