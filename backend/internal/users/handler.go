package users

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	Queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler {
	return &Handler{Queries: queries}
}

type createRequest struct {
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

type updateRequest struct {
	IsActive *bool  `json:"is_active"`
	Password string `json:"password"`
}

type response struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Role          string `json:"role"`
	Status        string `json:"status"`
	IsActive      bool   `json:"is_active"`
	InvitationURL string `json:"invitation_url,omitempty"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Phone = auth.NormalizePhone(req.Phone)
	req.Role = strings.TrimSpace(req.Role)
	if req.Name == "" || req.Phone == "" {
		api.WriteError(w, http.StatusBadRequest, "REQUIRED_FIELDS", "name and phone are required")
		return
	}
	if req.Role != "staff" && req.Role != "admin" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ROLE", "role must be staff or admin")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	if claims.Role == "admin" && req.Role != "staff" {
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "admins may only invite staff")
		return
	}

	rawToken, tokenHash, err := auth.NewInvitationToken()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to create invitation")
		return
	}
	placeholder, _, err := auth.NewInvitationToken()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to create invitation")
		return
	}
	hash, err := auth.HashPassword(placeholder)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to create invitation")
		return
	}
	user, err := h.Queries.CreateInvitedUser(r.Context(), db.CreateInvitedUserParams{
		Name: req.Name, Phone: req.Phone, PasswordHash: hash, Role: db.UserRole(req.Role),
		InvitationTokenHash: pgtype.Text{String: tokenHash, Valid: true},
		InvitationExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(auth.InvitationLifetime), Valid: true},
		CompanyID:           companyID,
	})
	if err != nil {
		api.WriteError(w, http.StatusConflict, "USER_CREATE_FAILED", "phone is already in use or user could not be created")
		return
	}

	writeUser(w, http.StatusCreated, invitationResponse(user.ID.String(), user.Name, user.Phone, string(user.Role), rawToken))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	items, err := h.Queries.ListUsers(r.Context(), companyID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "USERS_FETCH_FAILED", "failed to list users")
		return
	}
	result := make([]response, 0, len(items))
	for _, item := range items {
		status := "inactive"
		if item.IsActive {
			status = "active"
		} else if item.InvitationPending.Valid && item.InvitationPending.Bool {
			status = "invited"
		}
		result = append(result, response{ID: item.ID.String(), Name: item.Name, Phone: item.Phone, Role: string(item.Role), Status: status, IsActive: item.IsActive})
	}
	writeUser(w, http.StatusOK, result)
}

func (h *Handler) ResendInvitation(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user id")
		return
	}
	rawToken, tokenHash, err := auth.NewInvitationToken()
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to create invitation")
		return
	}
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	user, err := h.Queries.UpdateInvitation(r.Context(), db.UpdateInvitationParams{
		ID: pgtype.UUID{Bytes: id, Valid: true}, InvitationTokenHash: pgtype.Text{String: tokenHash, Valid: true},
		InvitationExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(auth.InvitationLifetime), Valid: true},
		CompanyID:           companyID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusNotFound, "INVITATION_NOT_FOUND", "invited user not found")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to create invitation")
		return
	}
	writeUser(w, http.StatusOK, invitationResponse(user.ID.String(), user.Name, user.Phone, string(user.Role), rawToken))
}

func (h *Handler) RevokeInvitation(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user id")
		return
	}
	result, err := h.Queries.RevokeInvitation(r.Context(), db.RevokeInvitationParams{ID: pgtype.UUID{Bytes: id, Valid: true}, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVITATION_FAILED", "failed to revoke invitation")
		return
	}
	if result.RowsAffected() == 0 {
		api.WriteError(w, http.StatusNotFound, "INVITATION_NOT_FOUND", "invited user not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func invitationResponse(id, name, phone, role, rawToken string) response {
	baseURL := strings.TrimRight(os.Getenv("INVITATION_BASE_URL"), "/")
	if baseURL == "" {
		baseURL = "http://localhost:5173"
	}
	return response{ID: id, Name: name, Phone: phone, Role: role, Status: "invited", InvitationURL: baseURL + "/invite/" + rawToken}
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user id")
		return
	}
	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if req.IsActive == nil && strings.TrimSpace(req.Password) == "" {
		api.WriteError(w, http.StatusBadRequest, "NO_UPDATE", "provide is_active or password")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	if req.IsActive != nil && !*req.IsActive && claims.UserID == id.String() {
		api.WriteError(w, http.StatusBadRequest, "SELF_DEACTIVATION_FORBIDDEN", "you cannot deactivate your own account")
		return
	}

	var item any
	if req.IsActive != nil {
		updated, updateErr := h.Queries.UpdateUserActive(r.Context(), db.UpdateUserActiveParams{ID: pgtype.UUID{Bytes: id, Valid: true}, IsActive: *req.IsActive, CompanyID: companyID})
		if updateErr != nil {
			writeUserUpdateError(w, updateErr)
			return
		}
		item = updated
	}
	if password := strings.TrimSpace(req.Password); password != "" {
		hash, hashErr := auth.HashPassword(password)
		if hashErr != nil {
			api.WriteError(w, http.StatusInternalServerError, "PASSWORD_HASH_FAILED", "failed to secure password")
			return
		}
		updated, updateErr := h.Queries.ResetUserPassword(r.Context(), db.ResetUserPasswordParams{ID: pgtype.UUID{Bytes: id, Valid: true}, PasswordHash: hash, CompanyID: companyID})
		if updateErr != nil {
			writeUserUpdateError(w, updateErr)
			return
		}
		item = updated
	}
	writeUser(w, http.StatusOK, responseFromUpdate(item))
}

func responseFromUpdate(item any) response {
	switch user := item.(type) {
	case db.UpdateUserActiveRow:
		return response{ID: user.ID.String(), Name: user.Name, Phone: user.Phone, Role: string(user.Role), Status: statusFor(user.IsActive, false), IsActive: user.IsActive}
	case db.ResetUserPasswordRow:
		return response{ID: user.ID.String(), Name: user.Name, Phone: user.Phone, Role: string(user.Role), Status: statusFor(user.IsActive, false), IsActive: user.IsActive}
	default:
		return response{}
	}
}

func statusFor(active, invited bool) string {
	if active {
		return "active"
	}
	if invited {
		return "invited"
	}
	return "inactive"
}

func writeUser(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeUserUpdateError(w http.ResponseWriter, err error) {
	if err == pgx.ErrNoRows {
		api.WriteError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}
	api.WriteError(w, http.StatusInternalServerError, "USER_UPDATE_FAILED", "failed to update user")
}
