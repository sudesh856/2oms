package couriers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"oms-backend/internal/auth"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
)

func TestStaffCannotListCouriersThroughJWTMiddleware(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		t.Skip("JWT_SECRET is not set")
	}

	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	token, err := auth.GenerateToken("test-staff-user", "staff", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/couriers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler := NewHandler(db.New(pool))
	auth.RequireAuth(jwtSecret)(auth.RequireRole("superadmin", "admin")(http.HandlerFunc(handler.ListCouriers))).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected staff courier access to return 403, got %d: %s", rec.Code, rec.Body.String())
	}
}
