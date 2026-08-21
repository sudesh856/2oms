package auth

import (
	"context"
	"net/http"
	"strings"

	"oms-backend/internal/api"
	db "oms-backend/internal/db/generated"

	"github.com/jackc/pgx/v5/pgtype"
)

type contextKey string

const claimsKey contextKey = "claims"

func RequireAuth(secret string, queryOptions ...*db.Queries) func(http.Handler) http.Handler {
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

			if len(queryOptions) > 0 && queryOptions[0] != nil {
				userID := pgtype.UUID{}
				if err := userID.Scan(claims.UserID); err != nil {
					api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
					return
				}
				user, err := queryOptions[0].GetUserAuthContext(r.Context(), userID)
				if err != nil || !user.IsActive {
					api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
					return
				}
				claims.Role = string(user.Role)
				claims.CompanyID = user.CompanyID.String()
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

func GetCompanyID(ctx context.Context) (pgtype.UUID, bool) {
	claims, ok := GetClaims(ctx)
	if !ok {
		return pgtype.UUID{}, false
	}
	id := pgtype.UUID{}
	if err := id.Scan(claims.CompanyID); err != nil {
		return pgtype.UUID{}, false
	}
	return id, true
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
