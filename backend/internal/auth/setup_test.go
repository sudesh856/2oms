package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func TestSetupCreatesOnlyTheInitialSuperadmin(t *testing.T) {
	_ = godotenv.Load("../../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	if os.Getenv("JWT_SECRET") == "" {
		t.Skip("JWT_SECRET is not set")
	}

	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	tx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(context.Background(), "CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS) ON COMMIT PRESERVE ROWS"); err != nil {
		t.Fatalf("create isolated users table: %v", err)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	handler := &Handler{
		Queries: db.New(tx),
		BeginTx: func(ctx context.Context) (pgx.Tx, error) {
			return conn.Begin(ctx)
		},
	}
	setupBody := `{"name":"First Owner","phone":" 9812345678 ","password":"OwnerPass123!","role":"admin"}`
	setupRequest := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(setupBody))
	setupRequest.Header.Set("Content-Type", "application/json")
	setupRecording := httptest.NewRecorder()
	handler.Setup(setupRecording, setupRequest)
	if setupRecording.Code != http.StatusOK {
		t.Fatalf("initial setup: expected 200, got %d: %s", setupRecording.Code, setupRecording.Body.String())
	}

	checkTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var role string
	if err := checkTx.QueryRow(context.Background(), "SELECT role::text FROM users LIMIT 1").Scan(&role); err != nil {
		t.Fatalf("read initial role: %v", err)
	}
	checkTx.Rollback(context.Background())
	if role != "superadmin" {
		t.Fatalf("expected server-assigned superadmin role, got %s", role)
	}

	var tokenResponse loginResponse
	if err := json.NewDecoder(setupRecording.Body).Decode(&tokenResponse); err != nil || tokenResponse.Token == "" {
		t.Fatalf("expected setup token, got %s", setupRecording.Body.String())
	}

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"9812345678","password":"OwnerPass123!"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginRecording := httptest.NewRecorder()
	loginTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	loginHandler := &Handler{Queries: db.New(loginTx)}
	loginHandler.Login(loginRecording, loginRequest)
	loginTx.Rollback(context.Background())
	if loginRecording.Code != http.StatusOK {
		t.Fatalf("login after setup: expected 200, got %d: %s", loginRecording.Code, loginRecording.Body.String())
	}

	secondRequest := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"name":"Second Owner","phone":"9823456789","password":"AnotherPass123!"}`))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRecording := httptest.NewRecorder()
	handler.Setup(secondRecording, secondRequest)
	if secondRecording.Code != http.StatusConflict {
		t.Fatalf("second setup: expected 409, got %d: %s", secondRecording.Code, secondRecording.Body.String())
	}

	authenticatedRequest := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(setupBody))
	authenticatedRequest.Header.Set("Authorization", "Bearer already-authenticated")
	authenticatedRecording := httptest.NewRecorder()
	handler.Setup(authenticatedRecording, authenticatedRequest)
	if authenticatedRecording.Code != http.StatusForbidden {
		t.Fatalf("authenticated setup: expected 403, got %d: %s", authenticatedRecording.Code, authenticatedRecording.Body.String())
	}
}
