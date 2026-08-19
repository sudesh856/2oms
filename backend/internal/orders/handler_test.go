package orders

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

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

func TestStaffOrderListSearchesCustomerPhoneWithoutCOD(t *testing.T) {
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

	var phone string
	var name string
	var status string
	var source string
	var createdAt time.Time
	var courierID string
	err = pool.QueryRow(
		context.Background(),
		`SELECT c.phone, c.name, o.status, o.source, o.created_at,
			COALESCE(o.courier_id::text, '')
		 FROM orders o
		 JOIN customers c ON c.id = o.customer_id
		 LIMIT 1`,
	).Scan(&phone, &name, &status, &source, &createdAt, &courierID)
	if err != nil {
		t.Skip("no orders with customers available in database")
	}

	token, err := auth.GenerateToken("test-staff-user", "staff", jwtSecret)
	if err != nil {
		t.Fatalf("failed to generate JWT: %v", err)
	}

	handler := NewHandler(db.New(pool), pool)
	for _, search := range []string{phone, name} {
		req := httptest.NewRequest(http.MethodGet, "/api/orders?search="+url.QueryEscape(search)+"&limit=1", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		auth.RequireAuth(jwtSecret)(http.HandlerFunc(handler.ListOrders)).ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %q, got %d: %s", search, rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"orders"`) || !strings.Contains(body, `"pagination"`) {
			t.Fatalf("expected paginated order response, got %s", body)
		}
		var response struct {
			Orders []json.RawMessage `json:"orders"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode order response: %v", err)
		}
		if len(response.Orders) == 0 {
			t.Fatalf("expected search %q to return the existing order", search)
		}
		if strings.Contains(body, "cod_amount") {
			t.Fatalf("STAFF order list leaked cod_amount: %s", body)
		}
	}

	filters := url.Values{
		"status":    []string{status},
		"source":    []string{source},
		"from_date": []string{createdAt.UTC().Format("2006-01-02")},
		"to_date":   []string{createdAt.UTC().Format("2006-01-02")},
		"limit":     []string{"1"},
	}
	if courierID != "" {
		filters.Set("courier_id", courierID)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/orders?"+filters.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	auth.RequireAuth(jwtSecret)(http.HandlerFunc(handler.ListOrders)).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected filtered order list 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var filteredResponse struct {
		Orders []json.RawMessage `json:"orders"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &filteredResponse); err != nil {
		t.Fatalf("decode filtered order response: %v", err)
	}
	if len(filteredResponse.Orders) == 0 {
		t.Fatalf("expected combined status/date/source/courier filters to return the existing order")
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
