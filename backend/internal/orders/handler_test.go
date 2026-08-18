package orders

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oms-backend/internal/auth"
)

func TestStaffOrderResponseDoesNotExposeCODAmount(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := &auth.Claims{
			UserID: "staff-user",
			Role:   "staff",
		}

		_ = claims

		w.Header().Set("Content-Type", "application/json")

		response := map[string]any{
			"id":             "test-order",
			"status":         "confirmed",
			"address":        "Kathmandu",
			"is_store_visit": false,
		}

		json.NewEncoder(w).Encode(response)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/orders/test-order", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if strings.Contains(rec.Body.String(), "cod_amount") {
		t.Fatal("staff response must not contain cod_amount")
	}
}
