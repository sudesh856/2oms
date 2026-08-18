package auth

import (
	"context"
	"net/http"
	"strings"

	"oms-backend/internal/api"
)

type contextKey string

const claimsKey contextKey = "claims"

func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			if !strings.HasPrefix(header, "Bearer ") {
				api.WriteError(
					w,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"unauthorized",
				)
				return
			}

			tokenString := strings.TrimPrefix(header, "Bearer ")

			claims, err := ParseToken(tokenString, secret)
			if err != nil {
				api.WriteError(
					w,
					http.StatusUnauthorized,
					"UNAUTHORIZED",
					"unauthorized",
				)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				claimsKey,
				claims,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetClaims(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(claimsKey).(*Claims)
	return claims, ok
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool)

	for _, role := range roles {
		allowed[role] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetClaims(r.Context())

			if !ok || !allowed[claims.Role] {
				api.WriteError(
					w,
					http.StatusForbidden,
					"FORBIDDEN",
					"forbidden",
				)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
