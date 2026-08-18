package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		role       string
		allowed    []string
		wantStatus int
	}{
		{
			name:       "admin allowed",
			role:       "admin",
			allowed:    []string{"admin", "superadmin"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "staff forbidden",
			role:       "staff",
			allowed:    []string{"admin", "superadmin"},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "superadmin allowed",
			role:       "superadmin",
			allowed:    []string{"admin", "superadmin"},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := RequireRole(tt.allowed...)(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
				}),
			)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req = req.WithContext(
				contextWithClaims(req.Context(), &Claims{
					UserID: "test-user",
					Role:   tt.role,
				}),
			)

			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func contextWithClaims(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}
