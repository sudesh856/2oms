package main

import (
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
	"oms-backend/internal/users"
)

func TestPhase3RoutesUseRealRouterAndRBAC(t *testing.T) {
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
	authHandler := &auth.Handler{Queries: queries}
	orderHandler := orders.NewHandler(queries, pool)
	customerHandler := customers.NewHandler(queries)
	productHandler := products.NewHandler(queries)
	courierHandler := couriers.NewHandler(queries)
	reportHandler := reports.NewHandler(queries)
	router := NewRouter(jwtSecret, authHandler, orderHandler, customerHandler, productHandler, courierHandler, reportHandler, users.NewHandler(queries))

	staffToken, err := auth.GenerateToken("test-staff-user", "staff", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate staff JWT: %v", err)
	}
	adminToken, err := auth.GenerateToken("test-admin-user", "admin", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate admin JWT: %v", err)
	}

	adminOnlyRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/couriers"},
		{http.MethodPost, "/api/couriers"},
		{http.MethodPatch, "/api/couriers/00000000-0000-0000-0000-000000000000"},
		{http.MethodDelete, "/api/couriers/00000000-0000-0000-0000-000000000000"},
		{http.MethodGet, "/api/couriers/00000000-0000-0000-0000-000000000000/locations"},
		{http.MethodPost, "/api/couriers/00000000-0000-0000-0000-000000000000/locations"},
		{http.MethodPatch, "/api/couriers/00000000-0000-0000-0000-000000000000/locations/00000000-0000-0000-0000-000000000000"},
		{http.MethodDelete, "/api/couriers/00000000-0000-0000-0000-000000000000/locations/00000000-0000-0000-0000-000000000000"},
		{http.MethodGet, "/api/reports/staff-performance"},
		{http.MethodGet, "/api/reports/confirmed-courier-wise"},
	}

	for _, route := range adminOnlyRoutes {
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+staffToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: expected staff 403 from registered router, got %d: %s", route.method, route.path, rec.Code, rec.Body.String())
		}
	}

	request := httptest.NewRequest(http.MethodGet, "/api/couriers", nil)
	request.Header.Set("Authorization", "Bearer "+adminToken)
	recording := httptest.NewRecorder()
	router.ServeHTTP(recording, request)
	if recording.Code != http.StatusOK {
		t.Fatalf("admin GET /api/couriers did not reach the real handler: %d: %s", recording.Code, recording.Body.String())
	}

	followUpRequest := httptest.NewRequest(http.MethodPost, "/api/orders/not-a-uuid/followup", strings.NewReader(`{}`))
	followUpRequest.Header.Set("Authorization", "Bearer "+staffToken)
	followUpRecording := httptest.NewRecorder()
	router.ServeHTTP(followUpRecording, followUpRequest)
	if followUpRecording.Code != http.StatusBadRequest {
		t.Fatalf("staff POST /api/orders/{id}/followup did not reach the real handler: %d: %s", followUpRecording.Code, followUpRecording.Body.String())
	}

	followUpsRequest := httptest.NewRequest(http.MethodGet, "/api/followups?due_today=true&unanswered=true", nil)
	followUpsRequest.Header.Set("Authorization", "Bearer "+staffToken)
	followUpsRecording := httptest.NewRecorder()
	router.ServeHTTP(followUpsRecording, followUpsRequest)
	if followUpsRecording.Code != http.StatusOK {
		t.Fatalf("staff GET /api/followups did not reach the real handler: %d: %s", followUpsRecording.Code, followUpsRecording.Body.String())
	}

	exportRequest := httptest.NewRequest(http.MethodGet, "/api/reports/orders.csv?status=confirmed", nil)
	exportRequest.Header.Set("Authorization", "Bearer "+staffToken)
	exportRecording := httptest.NewRecorder()
	router.ServeHTTP(exportRecording, exportRequest)
	if exportRecording.Code != http.StatusForbidden {
		t.Fatalf("staff CSV export should return 403 from the real router, got %d: %s", exportRecording.Code, exportRecording.Body.String())
	}

}

func TestAllowedOrigins(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://one.example, http://localhost:5173")
	origins := allowedOrigins()
	if len(origins) != 2 || origins[0] != "https://one.example" || origins[1] != "http://localhost:5173" {
		t.Fatalf("unexpected configured origins: %#v", origins)
	}

	t.Setenv("ALLOWED_ORIGINS", "")
	origins = allowedOrigins()
	if len(origins) != 1 || origins[0] != "https://frontend398745lkajsgd.onrender.com" {
		t.Fatalf("unexpected fallback origins: %#v", origins)
	}
}
