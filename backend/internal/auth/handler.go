package auth

import (
	"encoding/json"
	"net/http"
	"os"

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
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.Queries.GetUserByPhone(r.Context(), req.Phone)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := CheckPassword(req.Password, user.PasswordHash); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	secret := os.Getenv("JWT_SECRET")

	token, err := GenerateToken(
		user.ID.String(),
		string(user.Role),
		secret,
	)
	if err != nil {
		http.Error(w, "failed to generate token", http.StatusInternalServerError)
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
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	json.NewEncoder(w).Encode(map[string]string{
		"user_id": claims.UserID,
		"role":    claims.Role,
	})
}
