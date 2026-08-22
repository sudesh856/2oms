package main

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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
				"/api/orders/" + pair.own.orderID.String() + "/items",
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
				"/api/orders/" + pair.other.orderID.String() + "/items",
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
			status, body = tenantRequest(router, token, http.MethodPatch, "/api/orders/"+pair.other.orderID.String(), `{"courier_id":"`+pair.own.courierID.String()+`"}`)
			if status == http.StatusOK || strings.Contains(body, pair.other.name) {
				t.Errorf("cross-company order update succeeded: %d %s", status, body)
			}
		})
	}

	checkCompanyNotNull(t, pool)
}

func TestCreateOrderSetsOrderItemCompanyID(t *testing.T) {
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
	fixture, err := createTenantFixture(ctx, pool, "order-item-company")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupTenantFixtures(t, pool, fixture.companyID)

	router := newIntegrationRouterWithPool(t, pool, secret)
	token := mustTenantToken(t, fixture.userID, fixture.companyID, secret)
	status, body := tenantRequest(
		router,
		token,
		http.MethodPost,
		"/api/orders",
		`{"customer_id":"`+fixture.customerID.String()+`","source":"phone","address":"delivery address","cod_amount":"100","items":[{"product_id":"`+fixture.productID.String()+`","quantity":1}]}`,
	)
	if status != http.StatusCreated {
		t.Fatalf("order creation failed: %d %s", status, body)
	}

	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil || response.ID == "" {
		t.Fatalf("order response did not contain an id: %s", body)
	}
	var itemCompanyID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT company_id FROM order_items WHERE order_id = $1`, response.ID).Scan(&itemCompanyID); err != nil {
		t.Fatal(err)
	}
	if itemCompanyID != fixture.companyID {
		t.Fatalf("order item company_id = %s, expected %s", itemCompanyID, fixture.companyID)
	}
}

func TestLegacyImportCannotCrossTenantBoundary(t *testing.T) {
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
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'legacy_import_runs')").Scan(&migrated); err != nil {
		t.Fatal(err)
	}
	if !migrated {
		t.Skip("legacy import migration 000008 is not applied")
	}
	first, err := createTenantFixture(ctx, pool, "import-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := createTenantFixture(ctx, pool, "import-b")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupTenantFixtures(t, pool, first.companyID, second.companyID)
	if _, err := pool.Exec(ctx, "INSERT INTO legacy_import_runs (company_id, triggered_by, status) VALUES ($1, $2, 'completed')", first.companyID, first.userID); err != nil {
		t.Fatal(err)
	}
	router := newIntegrationRouterWithPool(t, pool, secret)
	firstToken := mustTenantToken(t, first.userID, first.companyID, secret)
	secondToken := mustTenantToken(t, second.userID, second.companyID, secret)
	status, body := tenantRequest(router, firstToken, http.MethodGet, "/api/imports/legacy", "")
	if status != http.StatusOK || !strings.Contains(body, first.companyID.String()) {
		t.Fatalf("company A could not read its import: %d %s", status, body)
	}
	status, body = tenantRequest(router, secondToken, http.MethodGet, "/api/imports/legacy", "")
	if status != http.StatusNotFound || strings.Contains(body, first.companyID.String()) {
		t.Fatalf("company B saw company A import: %d %s", status, body)
	}
	status, body = tenantRequest(router, secondToken, http.MethodPost, "/api/imports/legacy", `{"year":2025,"source":"phone","company_id":"`+first.companyID.String()+`"}`)
	if status != http.StatusAccepted {
		t.Fatalf("company B could not start its own import without trusting body company_id: %d %s", status, body)
	}
	var runCompanyID uuid.UUID
	if err := pool.QueryRow(ctx, "SELECT company_id FROM legacy_import_runs WHERE company_id = $1", second.companyID).Scan(&runCompanyID); err != nil {
		t.Fatal(err)
	}
	if runCompanyID != second.companyID {
		t.Fatalf("import was assigned to wrong company: %s", runCompanyID)
	}
	var firstStatus string
	if err := pool.QueryRow(ctx, "SELECT status FROM legacy_import_runs WHERE company_id = $1", first.companyID).Scan(&firstStatus); err != nil {
		t.Fatal(err)
	}
	if firstStatus != "completed" {
		t.Fatalf("company A import was affected by company B: %s", firstStatus)
	}
}

func TestLegacyImportRecoveryAllowsRetryButBlocksCompleted(t *testing.T) {
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
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'legacy_import_runs')").Scan(&migrated); err != nil || !migrated {
		t.Skip("legacy import migration 000008 is not applied")
	}
	first, err := createTenantFixture(ctx, pool, "recovery-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := createTenantFixture(ctx, pool, "recovery-b")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupTenantFixtures(t, pool, first.companyID, second.companyID)
	if _, err := pool.Exec(ctx, "INSERT INTO legacy_import_runs (company_id, triggered_by, status) VALUES ($1, $2, 'running'), ($3, $4, 'completed')", first.companyID, first.userID, second.companyID, second.userID); err != nil {
		t.Fatal(err)
	}
	if err := recoverInterruptedImports(ctx, db.New(pool)); err != nil {
		t.Fatal(err)
	}
	var status, message string
	if err := pool.QueryRow(ctx, "SELECT status, COALESCE(error_message, '') FROM legacy_import_runs WHERE company_id = $1", first.companyID).Scan(&status, &message); err != nil {
		t.Fatal(err)
	}
	if status != "failed" || message != "import interrupted by server restart" {
		t.Fatalf("running import was not recovered: %s %q", status, message)
	}
	router := newIntegrationRouterWithPool(t, pool, secret)
	token := mustTenantToken(t, first.userID, first.companyID, secret)
	statusCode, body := tenantRequest(router, token, http.MethodPost, "/api/imports/legacy", `{"year":2025,"source":"phone"}`)
	if statusCode != http.StatusAccepted {
		t.Fatalf("failed import could not be retried: %d %s", statusCode, body)
	}
	completedToken := mustTenantToken(t, second.userID, second.companyID, secret)
	statusCode, body = tenantRequest(router, completedToken, http.MethodPost, "/api/imports/legacy", `{"year":2025,"source":"phone"}`)
	if statusCode != http.StatusConflict || !strings.Contains(body, "IMPORT_ALREADY_EXISTS") {
		t.Fatalf("completed import was not blocked: %d %s", statusCode, body)
	}
}

func TestLegacyCourierLocationsAreNCMOnly(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required")
	}
	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	var migrated bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version >= 9 AND dirty = FALSE)").Scan(&migrated); err != nil {
		t.Skip("seed correction migration status is unavailable")
	}
	if !migrated {
		t.Skip("courier seed correction migration 000009 is not applied")
	}
	var nonNCM int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM courier_locations cl JOIN couriers c ON c.id = cl.courier_id AND c.company_id = cl.company_id JOIN companies co ON co.id = cl.company_id WHERE co.name = 'Default Company' AND c.name IN ('Upaya/Delivery Sansar', 'Pathao/Doorma')`).Scan(&nonNCM); err != nil {
		t.Fatal(err)
	}
	if nonNCM != 0 {
		t.Fatalf("original company still has %d non-NCM seeded locations", nonNCM)
	}
}

func TestMappedUploadIsTenantScoped(t *testing.T) {
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
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'import_uploads')").Scan(&migrated); err != nil || !migrated {
		t.Skip("mapped upload migration 000010 is not applied")
	}
	first, err := createTenantFixture(ctx, pool, "upload-a")
	if err != nil {
		t.Fatal(err)
	}
	second, err := createTenantFixture(ctx, pool, "upload-b")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupTenantFixtures(t, pool, first.companyID, second.companyID)
	router := newIntegrationRouterWithPool(t, pool, secret)
	firstToken := mustTenantToken(t, first.userID, first.companyID, secret)
	secondToken := mustTenantToken(t, second.userID, second.companyID, secret)
	uploadRequest := func(token, content string) (int, string) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, partErr := writer.CreateFormFile("orders", "orders.csv")
		if partErr != nil {
			t.Fatal(partErr)
		}
		_, _ = part.Write([]byte(content))
		_ = writer.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/imports/mapped/upload", &body)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	status, body := uploadRequest(firstToken, "Customer,Mobile,Street,Item,Amount\nAsha,9812345678,Kathmandu,Belt,1000\n")
	if status != http.StatusCreated {
		t.Fatalf("company A upload failed: %d %s", status, body)
	}
	var response struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil || response.ID == "" {
		t.Fatalf("company A upload did not return an ID: %s", body)
	}
	status, body = tenantRequest(router, secondToken, http.MethodGet, "/api/imports/mapped/"+response.ID+"/preview", "")
	if status != http.StatusNotFound || strings.Contains(body, "Customer") {
		t.Fatalf("company B accessed company A preview: %d %s", status, body)
	}
	status, body = tenantRequest(router, secondToken, http.MethodPut, "/api/imports/mapped/"+response.ID+"/mapping", `{"mapping":{"name":"Customer","phone":"Mobile","address":"Street","product":"Item","cod_amount":"Amount"}}`)
	if status != http.StatusNotFound {
		t.Fatalf("company B accessed company A mapping: %d %s", status, body)
	}
	status, body = uploadRequest(firstToken, "Customer,Mobile,Street,Item,Amount\n=cmd,9812345678,Kathmandu,Belt,1000\n")
	if status != http.StatusBadRequest || strings.Contains(body, "=cmd") {
		t.Fatalf("malicious upload was not rejected safely: %d %s", status, body)
	}
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
	tables := []string{"status_history", "follow_ups", "order_items", "orders", "courier_locations", "couriers", "products", "customers", "users"}
	var importRunsExists bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.legacy_import_runs') IS NOT NULL").Scan(&importRunsExists); err == nil && importRunsExists {
		tables = append([]string{"legacy_import_runs"}, tables...)
	}
	var uploadsExist bool
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.import_uploads') IS NOT NULL").Scan(&uploadsExist); err == nil && uploadsExist {
		tables = append([]string{"import_uploads"}, tables...)
	}
	for _, table := range tables {
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
	return NewRouter(secret, &auth.Handler{Queries: queries}, orders.NewHandler(queries, pool), customers.NewHandler(queries), products.NewHandler(queries), couriers.NewHandler(queries), reports.NewHandler(queries), users.NewHandler(queries), imports.NewHandler(queries, pool), queries)
}
