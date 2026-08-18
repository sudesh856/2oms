package auth

import (
	"encoding/json"
	"net/http"
	"os"

	"oms-backend/internal/api"
	db "oms-backend/internal/db/generated"
)

type Handler struct {
	Queries *db.Queries
}

type loginRequest struct {
	Phone    string `json:"phone"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	user, err := h.Queries.GetUserByPhone(r.Context(), req.Phone)
	if err != nil {
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
