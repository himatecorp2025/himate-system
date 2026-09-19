package httpapi

import (
	"context"

	"himate.local/backend/internal/auth"
)

type contextKey string

const claimsKey contextKey = "himate-auth-claims"

func withClaims(ctx context.Context, claims auth.Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

func claimsFromRequestContext(ctx context.Context) (auth.Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(auth.Claims)
	return claims, ok
}

func claimsFromRequest(r interface{ Context() context.Context }) (auth.Claims, bool) {
	return claimsFromRequestContext(r.Context())
}
