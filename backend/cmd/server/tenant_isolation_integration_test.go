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
	"oms-backend/internal/users"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type tenantFixture struct {
	companyID   uuid.UUID
	userID      uuid.UUID
	customerID  uuid.UUID
	productID   uuid.UUID
	courierID   uuid.UUID
	locationID  uuid.UUID
	orderID     uuid.UUID
	otherOrder  uuid.UUID
	phone       string
	name        string
	staffName   string
	courierName string
}

func TestTenantIsolationAcrossRegisteredResources(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	secret := os.Getenv("JWT_SECRET")
	if databaseURL == "" || secret == "" {
		t.Skip("DATABASE_URL and JWT_SECRET are required")
	}
	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	var migrated bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'companies')").Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Skip("tenant migration 000007 is not applied")
	}

	first, err := createTenantFixture(ctx, pool, "a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := createTenantFixture(ctx, pool, "b")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupTenantFixtures(t, pool, first.companyID, second.companyID)

	router := newIntegrationRouterWithPool(t, pool, secret)
	for _, pair := range []struct {
		name  string
		own   tenantFixture
		other tenantFixture
	}{
		{name: "company a", own: first, other: second},
		{name: "company b", own: second, other: first},
	} {
		t.Run(pair.name, func(t *testing.T) {
			token := mustTenantToken(t, pair.own.userID, pair.own.companyID, secret)
			ownPaths := []string{
				"/api/customers", "/api/customers/search?phone=" + pair.own.phone,
				"/api/products", "/api/couriers", "/api/couriers/" + pair.own.courierID.String() + "/locations",
				"/api/orders", "/api/followups", "/api/dashboard/summary", "/api/users",
				"/api/reports/orders.csv", "/api/reports/staff-performance", "/api/reports/confirmed-courier-wise",
			}
			for _, path := range ownPaths {
				status, body := tenantRequest(router, token, http.MethodGet, path, "")
				if status != http.StatusOK {
					t.Errorf("own GET %s: expected 200, got %d: %s", path, status, body)
				}
				if strings.Contains(body, pair.other.name) || strings.Contains(body, pair.other.phone) {
					t.Errorf("own GET %s leaked other company data: %s", path, body)
				}
			}
			status, body := tenantRequest(router, token, http.MethodGet, "/api/dashboard/summary", "")
			if status != http.StatusOK || !strings.Contains(body, `"confirmed_orders":1`) {
				t.Errorf("own dashboard did not report its confirmed order: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodGet, "/api/reports/staff-performance", "")
			if status != http.StatusOK || !strings.Contains(body, pair.own.staffName) || strings.Contains(body, pair.other.staffName) {
				t.Errorf("staff report crossed tenant boundary: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodGet, "/api/reports/confirmed-courier-wise", "")
			if status != http.StatusOK || !strings.Contains(body, pair.own.courierName) || strings.Contains(body, pair.other.courierName) {
				t.Errorf("courier report crossed tenant boundary: %d %s", status, body)
			}

			objectPaths := []string{
				"/api/customers/" + pair.own.customerID.String(),
				"/api/products/" + pair.own.productID.String(),
				"/api/orders/" + pair.own.orderID.String(),
				"/api/customers/" + pair.own.customerID.String() + "/history",
			}
			for _, path := range objectPaths {
				status, body := tenantRequest(router, token, http.MethodGet, path, "")
				if status != http.StatusOK {
					t.Errorf("own GET %s: expected 200, got %d: %s", path, status, body)
				}
			}

			idorPaths := []string{
				"/api/customers/" + pair.other.customerID.String(),
				"/api/products/" + pair.other.productID.String(),
				"/api/orders/" + pair.other.orderID.String(),
				"/api/customers/" + pair.other.customerID.String() + "/history",
				"/api/couriers/" + pair.other.courierID.String() + "/locations",
				"/api/couriers/" + pair.other.courierID.String() + "/locations/" + pair.other.locationID.String(),
			}
			for _, path := range idorPaths {
				status, body := tenantRequest(router, token, http.MethodGet, path, "")
				if status == http.StatusOK || strings.Contains(body, pair.other.name) || strings.Contains(body, pair.other.phone) {
					t.Errorf("IDOR GET %s succeeded or leaked data: %d %s", path, status, body)
				}
			}

			status, body = tenantRequest(router, token, http.MethodPatch, "/api/products/"+pair.other.productID.String(), `{"name":"cross-company","price":"1","available_qty":1,"warehouse_qty":1}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company product update succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodPatch, "/api/couriers/"+pair.other.courierID.String(), `{"name":"cross-company"}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company courier update succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodPatch, "/api/couriers/"+pair.other.courierID.String()+"/locations/"+pair.other.locationID.String(), `{"location_name":"cross-company","delivery_charge":"1"}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company location update succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodPatch, "/api/users/"+pair.other.userID.String(), `{"is_active":false}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.phone) {
				t.Errorf("cross-company user update succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodDelete, "/api/couriers/"+pair.other.courierID.String(), "")
			if status == http.StatusNoContent || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company courier delete succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodDelete, "/api/couriers/"+pair.other.courierID.String()+"/locations/"+pair.other.locationID.String(), "")
			if status == http.StatusNoContent || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company location delete succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodPost, "/api/users/"+pair.other.userID.String()+"/revoke-invitation", "")
			if status == http.StatusNoContent || strings.Contains(body, pair.other.phone) {
				t.Errorf("cross-company invitation revoke succeeded: %d %s", status, body)
			}
			status, body = tenantRequest(router, token, http.MethodPatch, "/api/orders/"+pair.other.orderID.String()+"/status", `{"status":"pickup_complete"}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company status update succeeded: %d %s", status, body)
			}
		})
	}

	checkCompanyNotNull(t, pool)
}

func createTenantFixture(ctx context.Context, pool *pgxpool.Pool, suffix string) (tenantFixture, error) {
	fixture := tenantFixture{
		companyID: uuid.New(), userID: uuid.New(), customerID: uuid.New(), productID: uuid.New(),
		courierID: uuid.New(), locationID: uuid.New(), orderID: uuid.New(), otherOrder: uuid.New(),
		phone: "9" + strings.ReplaceAll(uuid.NewString(), "-", "")[:9], name: "tenant-" + suffix + "-customer",
		staffName: "tenant-" + suffix + "-user", courierName: "tenant-" + suffix + "-courier",
	}
	hash, err := auth.HashPassword("TenantPass123!")
	if err != nil {
		return fixture, err
	}
	queries := []struct {
		query string
		args  []any
	}{
		{"INSERT INTO companies (id, name) VALUES ($1, $2)", []any{fixture.companyID, "tenant-" + suffix + "-" + uuid.NewString()}},
		{"INSERT INTO users (id, company_id, name, phone, password_hash, role) VALUES ($1, $2, $3, $4, $5, 'superadmin')", []any{fixture.userID, fixture.companyID, fixture.staffName, fixture.phone, hash}},
		{"INSERT INTO customers (id, company_id, phone, name) VALUES ($1, $2, $3, $4)", []any{fixture.customerID, fixture.companyID, fixture.phone, fixture.name}},
		{"INSERT INTO products (id, company_id, name, price, available_qty, warehouse_qty) VALUES ($1, $2, $3, 100, 10, 10)", []any{fixture.productID, fixture.companyID, "tenant-" + suffix + "-product"}},
		{"INSERT INTO couriers (id, company_id, name) VALUES ($1, $2, $3)", []any{fixture.courierID, fixture.companyID, fixture.courierName}},
		{"INSERT INTO courier_locations (id, company_id, courier_id, location_name, delivery_charge) VALUES ($1, $2, $3, $4, 10)", []any{fixture.locationID, fixture.companyID, fixture.courierID, "tenant-" + suffix + "-location"}},
		{"INSERT INTO orders (id, company_id, customer_id, source, status, courier_id, location_id, address, cod_amount, created_by, is_legacy) VALUES ($1, $2, $3, 'phone', 'confirmed', $4, $5, 'address', 100, $6, TRUE)", []any{fixture.orderID, fixture.companyID, fixture.customerID, fixture.courierID, fixture.locationID, fixture.userID}},
		{"INSERT INTO order_items (company_id, order_id, product_id, quantity, price) VALUES ($1, $2, $3, 1, 100)", []any{fixture.companyID, fixture.orderID, fixture.productID}},
		{"INSERT INTO status_history (company_id, order_id, to_status, changed_by) VALUES ($1, $2, 'confirmed', $3)", []any{fixture.companyID, fixture.orderID, fixture.userID}},
		{"INSERT INTO follow_ups (company_id, order_id, attempt_no, next_action, assigned_to) VALUES ($1, $2, 1, 'no_answer', $3)", []any{fixture.companyID, fixture.orderID, fixture.userID}},
	}
	for _, item := range queries {
		if _, err := pool.Exec(ctx, item.query, item.args...); err != nil {
			return fixture, err
		}
	}
	return fixture, nil
}

func cleanupTenantFixtures(t *testing.T, pool *pgxpool.Pool, companyIDs ...uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	for _, table := range []string{"status_history", "follow_ups", "order_items", "orders", "courier_locations", "couriers", "products", "customers", "users"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table+" WHERE company_id = ANY($1)", companyIDs); err != nil {
			t.Errorf("cleanup %s: %v", table, err)
		}
	}
	if _, err := pool.Exec(ctx, "DELETE FROM companies WHERE id = ANY($1)", companyIDs); err != nil {
		t.Errorf("cleanup companies: %v", err)
	}
}

func checkCompanyNotNull(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, table := range []string{"users", "customers", "products", "couriers", "courier_locations", "orders", "order_items", "follow_ups", "status_history"} {
		var nullable string
		err := pool.QueryRow(context.Background(), "SELECT is_nullable FROM information_schema.columns WHERE table_schema='public' AND table_name=$1 AND column_name='company_id'", table).Scan(&nullable)
		if err != nil {
			t.Fatalf("company_id metadata for %s: %v", table, err)
		}
		if nullable != "NO" {
			t.Errorf("%s.company_id is %s, expected NOT NULL", table, nullable)
		}
	}
}

func mustTenantToken(t *testing.T, userID, companyID uuid.UUID, secret string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID.String(), "superadmin", secret, companyID.String())
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func tenantRequest(router http.Handler, token, method, path, body string) (int, string) {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func newIntegrationRouterWithPool(t *testing.T, pool *pgxpool.Pool, secret string) http.Handler {
	t.Helper()
	queries := db.New(pool)
	return NewRouter(secret, &auth.Handler{Queries: queries}, orders.NewHandler(queries, pool), customers.NewHandler(queries), products.NewHandler(queries), couriers.NewHandler(queries), reports.NewHandler(queries), users.NewHandler(queries), queries)
}
