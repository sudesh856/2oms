package orders

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"oms-backend/internal/auth"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
)

func TestStaffOrderResponseDoesNotExposeCODAmount(t *testing.T) {
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

	queries := db.New(pool)
	handler := NewHandler(queries, pool)

	// Find an existing order.
	var orderID string

	err = pool.QueryRow(
		context.Background(),
		`SELECT id::text FROM orders LIMIT 1`,
	).Scan(&orderID)

	if err != nil {
		t.Skip("no orders available in database")
	}

	// Create a real staff JWT.
	token, err := auth.GenerateToken(
		"test-staff-user",
		"staff",
		jwtSecret,
	)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/orders/"+orderID,
		nil,
	)

	req.Header.Set("Authorization", "Bearer "+token)

	// Set the Chi route parameter.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", orderID)

	req = req.WithContext(
		context.WithValue(
			req.Context(),
			chi.RouteCtxKey,
			rctx,
		),
	)

	rec := httptest.NewRecorder()

	// Real authentication middleware -> real GetOrder handler.
	auth.RequireAuth(jwtSecret)(
		http.HandlerFunc(handler.GetOrder),
	).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}

	if body := rec.Body.String(); contains(body, "cod_amount") {
		t.Fatalf(
			"STAFF response leaked cod_amount: %s",
			body,
		)
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
