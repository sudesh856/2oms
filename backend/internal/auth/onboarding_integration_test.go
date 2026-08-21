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

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

func TestPublicSetupCreatesIndependentCompanySuperadmin(t *testing.T) {
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
	var companiesTable bool
	if err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='companies')").Scan(&companiesTable); err != nil {
		t.Fatal(err)
	}
	if !companiesTable {
		t.Skip("tenant migration 000007 is not applied")
	}

	companyName := "Onboarding " + uuid.NewString()
	phone := "9" + strings.ReplaceAll(uuid.NewString(), "-", "")[:9]
	password := "OnboardingPass123!"
	handler := &Handler{Queries: db.New(pool), Pool: pool}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/setup", strings.NewReader(`{"company_name":"`+companyName+`","name":"New Owner","phone":"`+phone+`","password":"`+password+`","company_id":"00000000-0000-0000-0000-000000000000","role":"staff"}`))
	request.Header.Set("Content-Type", "application/json")
	recording := httptest.NewRecorder()
	handler.Setup(recording, request)
	if recording.Code != http.StatusOK {
		t.Fatalf("onboarding: expected 200, got %d: %s", recording.Code, recording.Body.String())
	}
	var response loginResponse
	if err := json.NewDecoder(recording.Body).Decode(&response); err != nil || response.Token == "" {
		t.Fatalf("onboarding token missing: %s", recording.Body.String())
	}
	claims, err := ParseToken(response.Token, secret)
	if err != nil {
		t.Fatalf("parse onboarding token: %v", err)
	}
	var role string
	var companyID uuid.UUID
	var passwordHash string
	if err := pool.QueryRow(ctx, "SELECT role::text, company_id, password_hash FROM users WHERE id=$1", claims.UserID).Scan(&role, &companyID, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if role != "superadmin" {
		t.Fatalf("expected superadmin, got %s", role)
	}
	if companyID.String() != claims.CompanyID {
		t.Fatalf("token company %s does not match database company %s", claims.CompanyID, companyID)
	}
	if passwordHash == password || CheckPassword(password, passwordHash) != nil {
		t.Fatal("onboarding password was not securely hashed")
	}
	var companyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM companies WHERE id=$1 AND name=$2", companyID, companyName).Scan(&companyCount); err != nil {
		t.Fatal(err)
	}
	if companyCount != 1 {
		t.Fatalf("expected exactly one new company, got %d", companyCount)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM users WHERE id=$1", claims.UserID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM companies WHERE id=$1", companyID); err != nil {
		t.Fatal(err)
	}
}
