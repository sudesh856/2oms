package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"oms-backend/internal/auth"
	"oms-backend/internal/couriers"
	"oms-backend/internal/customers"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/orders"
	"oms-backend/internal/products"
	"oms-backend/internal/reports"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type rbacRouteCase struct {
	name       string
	method     string
	path       string
	body       string
	allowed    map[string]bool
	wantStatus int
}

func TestEveryRegisteredRouteEnforcesRBAC(t *testing.T) {
	router, jwtSecret, pool := newIntegrationRouter(t)
	defer pool.Close()

	roles := []string{"staff", "admin", "superadmin"}
	testCases := []rbacRouteCase{
		{name: "me", method: http.MethodGet, path: "/api/me", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "list customers", method: http.MethodGet, path: "/api/customers", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "create customer", method: http.MethodPost, path: "/api/customers", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "search customer", method: http.MethodGet, path: "/api/customers/search", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "get customer", method: http.MethodGet, path: "/api/customers/not-a-uuid", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "customer history", method: http.MethodGet, path: "/api/customers/not-a-uuid/history", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "list products", method: http.MethodGet, path: "/api/products", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "get product", method: http.MethodGet, path: "/api/products/not-a-uuid", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "create product", method: http.MethodPost, path: "/api/products", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "put product", method: http.MethodPut, path: "/api/products/not-a-uuid", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "patch product", method: http.MethodPatch, path: "/api/products/not-a-uuid", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "list orders", method: http.MethodGet, path: "/api/orders", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "get order", method: http.MethodGet, path: "/api/orders/not-a-uuid", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "create order", method: http.MethodPost, path: "/api/orders", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "update order status", method: http.MethodPatch, path: "/api/orders/not-a-uuid/status", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "create follow-up", method: http.MethodPost, path: "/api/orders/not-a-uuid/followup", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "list follow-ups", method: http.MethodGet, path: "/api/followups", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "dashboard summary", method: http.MethodGet, path: "/api/dashboard/summary", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "problem orders", method: http.MethodGet, path: "/api/orders/problems", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "export orders", method: http.MethodGet, path: "/api/reports/orders.csv", allowed: adminRoles(), wantStatus: http.StatusOK},
		{name: "list couriers", method: http.MethodGet, path: "/api/couriers", allowed: adminRoles(), wantStatus: http.StatusOK},
		{name: "create courier", method: http.MethodPost, path: "/api/couriers", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "update courier", method: http.MethodPatch, path: "/api/couriers/not-a-uuid", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "delete courier", method: http.MethodDelete, path: "/api/couriers/not-a-uuid", allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "list locations", method: http.MethodGet, path: "/api/couriers/not-a-uuid/locations", allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "create location", method: http.MethodPost, path: "/api/couriers/not-a-uuid/locations", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "update location", method: http.MethodPatch, path: "/api/couriers/not-a-uuid/locations/not-a-uuid", body: `{}`, allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "delete location", method: http.MethodDelete, path: "/api/couriers/not-a-uuid/locations/not-a-uuid", allowed: adminRoles(), wantStatus: http.StatusBadRequest},
		{name: "admin test", method: http.MethodGet, path: "/api/admin/test", allowed: adminRoles(), wantStatus: http.StatusOK},
		{name: "staff test", method: http.MethodGet, path: "/api/staff/test", allowed: allRoles(), wantStatus: http.StatusOK},
	}

	for _, testCase := range testCases {
		for _, role := range roles {
			t.Run(testCase.name+"/"+role, func(t *testing.T) {
				token, err := auth.GenerateToken(uuid.NewString(), role, jwtSecret)
				if err != nil {
					t.Fatalf("failed to generate token: %v", err)
				}

				req := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()

				router.ServeHTTP(rec, req)

				want := http.StatusForbidden
				if testCase.allowed[role] {
					want = testCase.wantStatus
				}
				if rec.Code != want {
					t.Fatalf("%s %s as %s: expected %d, got %d: %s", testCase.method, testCase.path, role, want, rec.Code, rec.Body.String())
				}
			})
		}
	}
}

func TestPublicRoutes(t *testing.T) {
	router, _, pool := newIntegrationRouter(t)
	defer pool.Close()

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "health", method: http.MethodGet, path: "/health", wantStatus: http.StatusOK},
		{name: "login", method: http.MethodPost, path: "/api/auth/login", body: `{}`, wantStatus: http.StatusUnauthorized},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			req := httptest.NewRequest(testCase.method, testCase.path, strings.NewReader(testCase.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			if rec.Code != testCase.wantStatus {
				t.Fatalf("%s %s: expected %d, got %d: %s", testCase.method, testCase.path, testCase.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestStaffOrderResponsesNeverContainCODAmount(t *testing.T) {
	router, jwtSecret, pool := newIntegrationRouter(t)
	defer pool.Close()

	var orderID uuid.UUID
	if err := pool.QueryRow(context.Background(), "SELECT id FROM orders ORDER BY created_at DESC LIMIT 1").Scan(&orderID); err != nil {
		t.Fatalf("expected a database order fixture for COD visibility testing: %v", err)
	}

	token, err := auth.GenerateToken(uuid.NewString(), "staff", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate staff token: %v", err)
	}

	paths := []string{
		"/api/orders/" + orderID.String(),
		"/api/orders?search=" + orderID.String(),
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("staff GET %s: expected 200, got %d: %s", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(rec.Body.String(), `"cod_amount"`) {
			t.Fatalf("staff GET %s exposed cod_amount: %s", path, rec.Body.String())
		}
	}
}

func newIntegrationRouter(t *testing.T) (*chi.Mux, string, *pgxpool.Pool) {
	t.Helper()
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
	queries := db.New(pool)
	router := NewRouter(jwtSecret, &auth.Handler{Queries: queries}, orders.NewHandler(queries, pool), customers.NewHandler(queries), products.NewHandler(queries), couriers.NewHandler(queries), reports.NewHandler(queries))
	return router, jwtSecret, pool
}

func allRoles() map[string]bool {
	return map[string]bool{"staff": true, "admin": true, "superadmin": true}
}

func adminRoles() map[string]bool {
	return map[string]bool{"admin": true, "superadmin": true}
}
