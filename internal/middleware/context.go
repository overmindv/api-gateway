package middleware

import "context"

type requestIDKey struct{}
type authKey struct{}

type AuthInfo struct {
	UserID string
	Token  string
	Roles  []string
	Err    error
}

func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)

	return value
}

func ContextWithAuth(ctx context.Context, info AuthInfo) context.Context {
	return context.WithValue(ctx, authKey{}, info)
}

func Auth(ctx context.Context) (AuthInfo, bool) {
	info, ok := ctx.Value(authKey{}).(AuthInfo)

	return info, ok
}
