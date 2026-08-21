package users

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"oms-backend/internal/auth"
	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type invitationCreateResponse struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	InvitationURL string `json:"invitation_url"`
}

func invitationRouteRequest(method, path, token, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := chi.NewRouteContext()
	ctx.URLParams.Add("token", token)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}

func TestInvitationLifecycle(t *testing.T) {
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
	conn, err := pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	setupTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := setupTx.Exec(context.Background(), "CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS) ON COMMIT PRESERVE ROWS"); err != nil {
		t.Fatal(err)
	}
	if err := setupTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}

	var companyID string
	if err := pool.QueryRow(context.Background(), "SELECT id::text FROM companies ORDER BY created_at ASC LIMIT 1").Scan(&companyID); err != nil {
		t.Skip("no company available in database")
	}
	creatorToken, err := auth.GenerateToken("00000000-0000-0000-0000-000000000001", "superadmin", secret, companyID)
	if err != nil {
		t.Fatal(err)
	}
	createTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	userHandler := NewHandler(db.New(createTx))
	createRequest := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(`{"name":"Invited Staff","phone":"9811111111","role":"staff"}`))
	createRequest.Header.Set("Authorization", "Bearer "+creatorToken)
	createRequest.Header.Set("Content-Type", "application/json")
	createRecording := httptest.NewRecorder()
	auth.RequireAuth(secret)(http.HandlerFunc(userHandler.Create)).ServeHTTP(createRecording, createRequest)
	if createRecording.Code != http.StatusCreated {
		t.Fatalf("create invitation: expected 201, got %d: %s", createRecording.Code, createRecording.Body.String())
	}
	var created invitationCreateResponse
	if err := json.NewDecoder(createRecording.Body).Decode(&created); err != nil || created.Status != "invited" || created.InvitationURL == "" {
		t.Fatalf("expected invited response with URL, got %s", createRecording.Body.String())
	}
	if err := createTx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	token := created.InvitationURL[strings.LastIndex(created.InvitationURL, "/")+1:]

	readTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	authHandler := &auth.Handler{Queries: db.New(readTx), BeginTx: func(ctx context.Context) (pgx.Tx, error) { return conn.Begin(ctx) }}
	invitationRecording := httptest.NewRecorder()
	authHandler.GetInvitation(invitationRecording, invitationRouteRequest(http.MethodGet, "/api/auth/invitation/"+token, token, ""))
	if invitationRecording.Code != http.StatusOK {
		t.Fatalf("inspect invitation: expected 200, got %d: %s", invitationRecording.Code, invitationRecording.Body.String())
	}
	readTx.Rollback(context.Background())

	acceptRecording := httptest.NewRecorder()
	authHandler.AcceptInvitation(acceptRecording, invitationRouteRequest(http.MethodPost, "/api/auth/accept-invitation/"+token, token, `{"password":"StaffPass123!"}`))
	if acceptRecording.Code != http.StatusOK {
		t.Fatalf("accept invitation: expected 200, got %d: %s", acceptRecording.Code, acceptRecording.Body.String())
	}

	reuseRecording := httptest.NewRecorder()
	authHandler.AcceptInvitation(reuseRecording, invitationRouteRequest(http.MethodPost, "/api/auth/accept-invitation/"+token, token, `{"password":"AnotherPass123!"}`))
	if reuseRecording.Code != http.StatusGone {
		t.Fatalf("reuse invitation: expected 410, got %d: %s", reuseRecording.Code, reuseRecording.Body.String())
	}

	loginTx, err := conn.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	loginHandler := &auth.Handler{Queries: db.New(loginTx)}
	loginRecording := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"phone":"9811111111","password":"StaffPass123!"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	loginHandler.Login(loginRecording, loginRequest)
	if loginRecording.Code != http.StatusOK {
		t.Fatalf("login after invitation: expected 200, got %d: %s", loginRecording.Code, loginRecording.Body.String())
	}
	loginTx.Rollback(context.Background())
}
