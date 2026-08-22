package main

import (
	"context"
	"encoding/json"
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
	"oms-backend/internal/imports"
	"oms-backend/internal/orders"
	"oms-backend/internal/products"
	"oms-backend/internal/reports"
	"oms-backend/internal/users"

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
		{name: "get order items", method: http.MethodGet, path: "/api/orders/not-a-uuid/items", allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "create order", method: http.MethodPost, path: "/api/orders", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "update order status", method: http.MethodPatch, path: "/api/orders/not-a-uuid/status", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "create follow-up", method: http.MethodPost, path: "/api/orders/not-a-uuid/followup", body: `{}`, allowed: allRoles(), wantStatus: http.StatusBadRequest},
		{name: "list follow-ups", method: http.MethodGet, path: "/api/followups", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "dashboard summary", method: http.MethodGet, path: "/api/dashboard/summary", allowed: allRoles(), wantStatus: http.StatusOK},
		{name: "legacy import status", method: http.MethodGet, path: "/api/imports/legacy", allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusNotFound},
		{name: "legacy import start", method: http.MethodPost, path: "/api/imports/legacy", body: `{}`, allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "mapped import upload", method: http.MethodPost, path: "/api/imports/mapped/upload", allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "mapped import preview", method: http.MethodGet, path: "/api/imports/mapped/not-a-uuid/preview", allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "mapped import review", method: http.MethodGet, path: "/api/imports/mapped/not-a-uuid/review", allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "mapped import mapping", method: http.MethodPut, path: "/api/imports/mapped/not-a-uuid/mapping", body: `{}`, allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "mapped import start", method: http.MethodPost, path: "/api/imports/mapped/not-a-uuid/start", body: `{}`, allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
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
		{name: "list users", method: http.MethodGet, path: "/api/users", allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusOK},
		{name: "create user", method: http.MethodPost, path: "/api/users", body: `{}`, allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
		{name: "update user", method: http.MethodPatch, path: "/api/users/not-a-uuid", body: `{}`, allowed: map[string]bool{"superadmin": true}, wantStatus: http.StatusBadRequest},
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

func TestSuperadminManagesInternalUsers(t *testing.T) {
	router, jwtSecret, pool := newIntegrationRouter(t)
	defer pool.Close()

	superadminID := uuid.New()
	if err := pool.QueryRow(context.Background(), "SELECT id FROM users WHERE role = 'superadmin' LIMIT 1").Scan(&superadminID); err != nil {
		t.Fatalf("expected a superadmin fixture: %v", err)
	}
	staffPhone := "97" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	adminPhone := "98" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	defer pool.Exec(context.Background(), "DELETE FROM users WHERE phone IN ($1, $2)", staffPhone, adminPhone)

	superadminToken, err := auth.GenerateToken(superadminID.String(), "superadmin", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate superadmin token: %v", err)
	}

	create := func(name, phone, password, role string) (int, userResponse) {
		req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"`+name+`","phone":"`+phone+`","password":"`+password+`","role":"`+role+`"}`))
		req.Header.Set("Authorization", "Bearer "+superadminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var user userResponse
		_ = json.NewDecoder(rec.Body).Decode(&user)
		return rec.Code, user
	}

	if status, user := create("Real Staff", staffPhone, "StaffReal123!", "staff"); status != http.StatusCreated || user.Role != "staff" || user.IsActive == false {
		t.Fatalf("create staff: expected active staff 201, got %d %+v", status, user)
	}
	if status, user := create("Real Admin", adminPhone, "AdminReal123!", "admin"); status != http.StatusCreated || user.Role != "admin" || user.IsActive == false {
		t.Fatalf("create admin: expected active admin 201, got %d %+v", status, user)
	}
	if status, _ := create("Invalid Superadmin", "96"+strings.ReplaceAll(uuid.NewString(), "-", "")[:8], "Blocked123!", "superadmin"); status != http.StatusBadRequest {
		t.Fatalf("create superadmin: expected 400, got %d", status)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	listRequest.Header.Set("Authorization", "Bearer "+superadminToken)
	listRecording := httptest.NewRecorder()
	router.ServeHTTP(listRecording, listRequest)
	if listRecording.Code != http.StatusOK || strings.Contains(listRecording.Body.String(), "password_hash") {
		t.Fatalf("list users exposed an unsafe response: %d %s", listRecording.Code, listRecording.Body.String())
	}

	var staff userResponse
	if err := pool.QueryRow(context.Background(), "SELECT id, name, phone, role, is_active FROM users WHERE phone = $1", staffPhone).Scan(&staff.ID, &staff.Name, &staff.Phone, &staff.Role, &staff.IsActive); err != nil {
		t.Fatalf("find created staff: %v", err)
	}

	patch := func(id, body string) int {
		req := httptest.NewRequest(http.MethodPatch, "/api/users/"+id, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+superadminToken)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if status := patch(staff.ID, `{"is_active":false}`); status != http.StatusOK {
		t.Fatalf("deactivate staff: expected 200, got %d", status)
	}

	login := func(phone, password string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"`+phone+`","password":"`+password+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}
	if status := login(staffPhone, "StaffReal123!"); status != http.StatusUnauthorized {
		t.Fatalf("inactive staff login: expected 401, got %d", status)
	}
	if status := patch(staff.ID, `{"password":"StaffReset123!"}`); status != http.StatusOK {
		t.Fatalf("reset staff password: expected 200, got %d", status)
	}
	if status := patch(staff.ID, `{"is_active":true}`); status != http.StatusOK {
		t.Fatalf("reactivate staff: expected 200, got %d", status)
	}
	if status := login(staffPhone, "StaffReset123!"); status != http.StatusOK {
		t.Fatalf("new staff password login: expected 200, got %d", status)
	}

	if status := patch(superadminID.String(), `{"is_active":false}`); status != http.StatusBadRequest {
		t.Fatalf("self-deactivation: expected 400, got %d", status)
	}
}

type userResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	IsActive bool   `json:"is_active"`
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
	router := NewRouter(jwtSecret, &auth.Handler{Queries: queries}, orders.NewHandler(queries, pool), customers.NewHandler(queries), products.NewHandler(queries), couriers.NewHandler(queries), reports.NewHandler(queries), users.NewHandler(queries), imports.NewHandler(queries, pool))
	return router, jwtSecret, pool
}

func allRoles() map[string]bool {
	return map[string]bool{"staff": true, "admin": true, "superadmin": true}
}

func adminRoles() map[string]bool {
	return map[string]bool{"admin": true, "superadmin": true}
}
