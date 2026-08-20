package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLoginRateLimiterReturns429AfterBurst(t *testing.T) {
	limiter := NewLoginRateLimiter(0, 2)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 3; attempt++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		want := http.StatusNoContent
		if attempt == 3 {
			want = http.StatusTooManyRequests
		}
		if rec.Code != want {
			t.Fatalf("attempt %d: expected %d, got %d", attempt, want, rec.Code)
		}
	}
}
