package orders

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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
	var companyID string

	err = pool.QueryRow(
		context.Background(),
		`SELECT id::text, company_id::text FROM orders LIMIT 1`,
	).Scan(&orderID, &companyID)

	if err != nil {
		t.Skip("no orders available in database")
	}

	// Create a real staff JWT.
	token, err := auth.GenerateToken(
		"test-staff-user",
		"staff",
		jwtSecret,
		companyID,
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

	if body := rec.Body.String(); contains(body, `"cod_amount"`) {
		t.Fatalf(
			"STAFF response leaked cod_amount: %s",
			body,
		)
	}
}

func TestAdminOrderResponseIncludesCODAmount(t *testing.T) {
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

	var orderID string
	var companyID string

	err = pool.QueryRow(
		context.Background(),
		`SELECT id::text, company_id::text FROM orders LIMIT 1`,
	).Scan(&orderID, &companyID)

	if err != nil {
		t.Skip("no orders available in database")
	}

	token, err := auth.GenerateToken(
		"test-admin-user",
		"admin",
		jwtSecret,
		companyID,
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

	if body := rec.Body.String(); !contains(body, `"cod_amount"`) {
		t.Fatalf(
			"ADMIN response missing cod_amount: %s",
			body,
		)
	}
}

func TestGetOrderItems(t *testing.T) {
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

	var orderID string
	var companyID string

	err = pool.QueryRow(
		context.Background(),
		`SELECT id::text, company_id::text FROM orders LIMIT 1`,
	).Scan(&orderID, &companyID)

	if err != nil {
		t.Skip("no orders available in database")
	}

	token, err := auth.GenerateToken(
		"test-admin-user",
		"admin",
		jwtSecret,
		companyID,
	)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/orders/"+orderID+"/items",
		nil,
	)

	req.Header.Set("Authorization", "Bearer "+token)

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

	auth.RequireAuth(jwtSecret)(
		http.HandlerFunc(handler.GetOrderItems),
	).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf(
			"expected 200, got %d: %s",
			rec.Code,
			rec.Body.String(),
		)
	}
}

func TestCreateAndPatchOrderWithCourierAndLocation(t *testing.T) {
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

	var userID string
	var companyID string
	var customerID string
	var courierID string
	var locationID string
	var productID string

	ctx := context.Background()

	err = pool.QueryRow(ctx, `
		SELECT u.id::text, c.id::text, cu.id::text, co.id::text, cl.id::text, p.id::text
		FROM users u
		JOIN companies c ON c.id = u.company_id
		JOIN customers cu ON cu.company_id = c.id
		JOIN couriers co ON co.company_id = c.id
		JOIN courier_locations cl ON cl.courier_id = co.id AND cl.company_id = c.id
		JOIN products p ON p.company_id = c.id
		WHERE u.role IN ('admin', 'superadmin')
		LIMIT 1
	`).Scan(&userID, &companyID, &customerID, &courierID, &locationID, &productID)

	if err != nil {
		t.Skip("required seed data not found in database")
	}

	token, err := auth.GenerateToken(
		userID,
		"admin",
		jwtSecret,
		companyID,
	)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	createPayload := fmt.Sprintf(`{
		"customer_id": "%s",
		"source": "website",
		"address": "Kathmandu, Nepal",
		"cod_amount": "250",
		"is_store_visit": false,
		"courier_id": "%s",
		"location_id": "%s",
		"items": [{"product_id": "%s", "quantity": 1}]
	}`, customerID, courierID, locationID, productID)

	req := httptest.NewRequest(http.MethodPost, "/api/orders", strings.NewReader(createPayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	auth.RequireAuth(jwtSecret)(
		http.HandlerFunc(handler.CreateOrder),
	).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !contains(body, courierID) || !contains(body, locationID) {
		t.Fatalf("created order response missing courier/location: %s", body)
	}

	var createdOrder struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &createdOrder); err != nil {
		t.Fatalf("failed to decode created order: %v", err)
	}

	patchPayload := fmt.Sprintf(`{
		"courier_id": "%s",
		"location_id": "%s"
	}`, courierID, locationID)

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/orders/"+createdOrder.ID, strings.NewReader(patchPayload))
	patchReq.Header.Set("Authorization", "Bearer "+token)
	patchReq.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", createdOrder.ID)
	patchReq = patchReq.WithContext(context.WithValue(patchReq.Context(), chi.RouteCtxKey, rctx))

	patchRec := httptest.NewRecorder()
	auth.RequireAuth(jwtSecret)(
		http.HandlerFunc(handler.UpdateOrder),
	).ServeHTTP(patchRec, patchReq)

	patchBody := patchRec.Body.String()
	if !contains(patchBody, courierID) || !contains(patchBody, locationID) {
		t.Fatalf("updated order response missing courier/location: %s", patchBody)
	}

	// Test clearing / unassigning courier and location
	clearPayload := `{"courier_id": null, "location_id": null}`
	clearReq := httptest.NewRequest(http.MethodPatch, "/api/orders/"+createdOrder.ID, strings.NewReader(clearPayload))
	clearReq.Header.Set("Authorization", "Bearer "+token)
	clearReq.Header.Set("Content-Type", "application/json")
	clearReq = clearReq.WithContext(context.WithValue(clearReq.Context(), chi.RouteCtxKey, rctx))

	clearRec := httptest.NewRecorder()
	auth.RequireAuth(jwtSecret)(
		http.HandlerFunc(handler.UpdateOrder),
	).ServeHTTP(clearRec, clearReq)

	if clearRec.Code != http.StatusOK {
		t.Fatalf("expected 200 on clear, got %d: %s", clearRec.Code, clearRec.Body.String())
	}

	var clearedOrder struct {
		CourierID  *string `json:"courier_id"`
		LocationID *string `json:"location_id"`
	}
	if err := json.Unmarshal(clearRec.Body.Bytes(), &clearedOrder); err != nil {
		t.Fatalf("failed to decode cleared order: %v", err)
	}
	if clearedOrder.CourierID != nil || clearedOrder.LocationID != nil {
		t.Fatalf("expected courier_id and location_id to be null/nil after clear, got courier: %v, loc: %v", clearedOrder.CourierID, clearedOrder.LocationID)
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


