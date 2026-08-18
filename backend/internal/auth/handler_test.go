package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/joho/godotenv"
)

func TestLogin(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		t.Fatal("JWT_SECRET is not set")
	}

	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	queries := db.New(pool)
	handler := &Handler{Queries: queries}

	t.Run("success", func(t *testing.T) {
		body := strings.NewReader(`{
			"phone": "9800000000",
			"password": "ChangeMe123!"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, rec.Code)
		}

		if !strings.Contains(rec.Body.String(), `"token"`) {
			t.Fatalf("expected token in response, got: %s", rec.Body.String())
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		body := strings.NewReader(`{
			"phone": "9800000000",
			"password": "WrongPassword!"
		}`)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
		req.Header.Set("Content-Type", "application/json")

		rec := httptest.NewRecorder()

		handler.Login(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

var _ = context.Background
