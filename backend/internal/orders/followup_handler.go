package orders

import (
	"encoding/json"
	"net/http"
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

type createFollowUpRequest struct {
	NextAction     string `json:"next_action"`
	PreferredDay   string `json:"preferred_day"`
	NextActionDate string `json:"next_action_date"`
	Note           string `json:"note"`
}

func (h *Handler) CreateFollowUp(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	orderUUID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id")
		return
	}
	assignedTo, err := uuid.Parse(claims.UserID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVALID_USER_ID", "invalid user id")
		return
	}

	var req createFollowUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.NextAction = strings.TrimSpace(req.NextAction)
	req.PreferredDay = strings.TrimSpace(req.PreferredDay)
	req.Note = strings.TrimSpace(req.Note)
	if req.NextAction != "no_answer" && req.NextAction != "call_again" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_NEXT_ACTION", "next_action must be no_answer or call_again")
		return
	}
	if len(req.Note) > 500 {
		api.WriteError(w, http.StatusBadRequest, "INVALID_NOTE", "note must be 500 characters or fewer")
		return
	}

	orderID := pgtype.UUID{Bytes: orderUUID, Valid: true}
	var current db.OrderStatus
	switch claims.Role {
	case "admin", "superadmin":
		order, fetchErr := h.Queries.GetOrderForAdmin(r.Context(), db.GetOrderForAdminParams{ID: orderID, CompanyID: companyID})
		if fetchErr != nil {
			writeFollowUpOrderError(w, fetchErr)
			return
		}
		current = order.Status
	case "staff":
		order, fetchErr := h.Queries.GetOrderForStaff(r.Context(), db.GetOrderForStaffParams{ID: orderID, CompanyID: companyID})
		if fetchErr != nil {
			writeFollowUpOrderError(w, fetchErr)
			return
		}
		current = order.Status
	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	if current != db.OrderStatusFollowUp && !CanTransition(string(current), string(db.OrderStatusFollowUp)) {
		api.WriteError(w, http.StatusBadRequest, "INVALID_STATUS_TRANSITION", "order cannot be marked for follow-up from its current status")
		return
	}

	date := pgtype.Date{}
	if req.NextActionDate != "" {
		parsed, parseErr := time.Parse("2006-01-02", req.NextActionDate)
		if parseErr != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_NEXT_ACTION_DATE", "invalid next action date")
			return
		}
		date = pgtype.Date{Time: parsed, Valid: true}
	}
	if req.NextAction == "call_again" && !date.Valid {
		api.WriteError(w, http.StatusBadRequest, "NEXT_ACTION_DATE_REQUIRED", "next action date is required for call_again")
		return
	}

	attempt, err := h.Queries.GetLatestFollowUpAttempt(r.Context(), db.GetLatestFollowUpAttemptParams{OrderID: orderID, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "FOLLOW_UP_FETCH_FAILED", "failed to get follow-up attempts")
		return
	}
	followUp, err := h.Queries.CreateFollowUp(r.Context(), db.CreateFollowUpParams{
		OrderID: orderID, AttemptNo: attempt + 1,
		NextAction:     pgtype.Text{String: req.NextAction, Valid: true},
		PreferredDay:   pgtype.Text{String: req.PreferredDay, Valid: req.PreferredDay != ""},
		NextActionDate: date, Note: pgtype.Text{String: req.Note, Valid: req.Note != ""},
		AssignedTo: pgtype.UUID{Bytes: assignedTo, Valid: true},
		CompanyID:  companyID,
	})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "FOLLOW_UP_CREATE_FAILED", "failed to create follow-up")
		return
	}
	if current != db.OrderStatusFollowUp {
		if err := NewService(h.Pool).UpdateStatus(r.Context(), orderID, current, db.OrderStatusFollowUp, pgtype.UUID{Bytes: assignedTo, Valid: true}, companyID); err != nil {
			api.WriteError(w, http.StatusInternalServerError, "FOLLOW_UP_STATUS_FAILED", "failed to update order status")
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(followUp)
}

func writeFollowUpOrderError(w http.ResponseWriter, err error) {
	if err == pgx.ErrNoRows {
		api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
		return
	}
	api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
}

func (h *Handler) ListFollowUps(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	date := pgtype.Date{}
	if r.URL.Query().Get("due_today") == "true" {
		now := time.Now()
		date = pgtype.Date{Time: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
	}
	unanswered := r.URL.Query().Get("unanswered") == "true"
	items, err := h.Queries.ListFollowUps(r.Context(), db.ListFollowUpsParams{Column1: date, Column2: unanswered, Column3: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "FOLLOW_UPS_FETCH_FAILED", "failed to list follow-ups")
		return
	}
	if items == nil {
		items = []db.ListFollowUpsRow{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}
