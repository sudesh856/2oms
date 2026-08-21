package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"oms-backend/internal/api"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Queries *db.Queries
	Pool    *pgxpool.Pool
	BeginTx func(context.Context) (pgx.Tx, error)
}

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

type setupRequest struct {
	CompanyName string `json:"company_name"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	Password    string `json:"password"`
}

type invitationPasswordRequest struct {
	Password string `json:"password"`
}

func NormalizePhone(phone string) string {
	return strings.TrimSpace(phone)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	user, err := h.Queries.GetUserByPhone(r.Context(), NormalizePhone(req.Phone))
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials")
		return
	}

	if !user.IsActive {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials")
		return
	}

	if err := CheckPassword(req.Password, user.PasswordHash); err != nil {
		api.WriteError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid credentials")
		return
	}

	secret := os.Getenv("JWT_SECRET")

	token, err := GenerateToken(
		user.ID.String(),
		string(user.Role),
		secret,
		user.CompanyID.String(),
	)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "failed to generate token")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(loginResponse{
		Token: token,
	})
}

func (h *Handler) SetupStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"available": true})
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.TrimSpace(r.Header.Get("Authorization")), "Bearer ") {
		api.WriteError(w, http.StatusForbidden, "SETUP_UNAVAILABLE", "initial setup is unavailable")
		return
	}

	var req setupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	req.CompanyName = strings.TrimSpace(req.CompanyName)
	req.Name = strings.TrimSpace(req.Name)
	req.Phone = NormalizePhone(req.Phone)
	req.Password = strings.TrimSpace(req.Password)
	if req.CompanyName == "" || req.Name == "" || req.Phone == "" || req.Password == "" {
		api.WriteError(w, http.StatusBadRequest, "REQUIRED_FIELDS", "company_name, name, phone, and password are required")
		return
	}
	if len(req.Password) < 8 {
		api.WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", "password must be at least 8 characters")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "failed to secure password")
		return
	}

	if h.Pool == nil && h.BeginTx == nil {
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}
	begin := h.BeginTx
	if begin == nil {
		begin = h.Pool.Begin
	}
	tx, err := begin(r.Context())
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}
	defer tx.Rollback(r.Context())

	if _, err := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(834271)"); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}
	queries := h.Queries.WithTx(tx)
	var companyID pgtype.UUID
	if err := tx.QueryRow(r.Context(), "INSERT INTO companies (name) VALUES ($1) RETURNING id", req.CompanyName).Scan(&companyID); err != nil {
		if strings.Contains(err.Error(), "companies_name_key") {
			api.WriteError(w, http.StatusConflict, "COMPANY_CREATE_FAILED", "company name is already in use")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}
	user, err := queries.CreateUser(r.Context(), db.CreateUserParams{
		Name: req.Name, Phone: req.Phone, PasswordHash: hash, Role: db.UserRoleSuperadmin,
		CompanyID: companyID,
	})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "SETUP_FAILED", "failed to create initial account")
		return
	}

	secret := os.Getenv("JWT_SECRET")
	token, err := GenerateToken(user.ID.String(), string(user.Role), secret, companyID.String())
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "TOKEN_GENERATION_FAILED", "failed to create session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(loginResponse{Token: token})
}

func (h *Handler) GetInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		api.WriteError(w, http.StatusNotFound, "INVITATION_NOT_FOUND", "invitation not found")
		return
	}

	invitation, err := h.Queries.GetInvitation(r.Context(), pgtype.Text{String: HashInvitationToken(token), Valid: true})
	if err != nil {
		api.WriteError(w, http.StatusGone, "INVITATION_INVALID", "invitation is invalid or expired")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"name": invitation.Name,
		"role": string(invitation.Role),
	})
}

func (h *Handler) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	var req invitationPasswordRequest
	if token == "" || json.NewDecoder(r.Body).Decode(&req) != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.Password = strings.TrimSpace(req.Password)
	if len(req.Password) < 8 {
		api.WriteError(w, http.StatusBadRequest, "WEAK_PASSWORD", "password must be at least 8 characters")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "failed to secure password")
		return
	}
	if h.Pool == nil && h.BeginTx == nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to activate account")
		return
	}
	begin := h.BeginTx
	if begin == nil {
		begin = h.Pool.Begin
	}
	tx, err := begin(r.Context())
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to activate account")
		return
	}
	defer tx.Rollback(r.Context())
	if _, err := tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock(834272)"); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to activate account")
		return
	}

	queries := h.Queries.WithTx(tx)
	invitation, err := queries.GetInvitation(r.Context(), pgtype.Text{String: HashInvitationToken(token), Valid: true})
	if err != nil {
		api.WriteError(w, http.StatusGone, "INVITATION_INVALID", "invitation is invalid or expired")
		return
	}
	if _, err := queries.AcceptInvitation(r.Context(), db.AcceptInvitationParams{
		ID: invitation.ID, PasswordHash: hash,
		InvitationTokenHash: pgtype.Text{String: HashInvitationToken(token), Valid: true},
		CompanyID:           invitation.CompanyID,
	}); err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusGone, "INVITATION_INVALID", "invitation is invalid or expired")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to activate account")
		return
	}
	if err := tx.Commit(r.Context()); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to activate account")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "account activated"})
}

func Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := GetClaims(r.Context())

	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"user_id": claims.UserID,
		"role":    claims.Role,
	})
}
