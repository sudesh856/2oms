package reports

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct{ Queries *db.Queries }

func NewHandler(queries *db.Queries) *Handler { return &Handler{Queries: queries} }

type dashboardResponse struct {
	TodayOrders          int32                              `json:"today_orders"`
	PendingConfirmations int32                              `json:"pending_confirmations"`
	ProblemOrders        int32                              `json:"problem_orders"`
	TotalOrders          int32                              `json:"total_orders"`
	StatusCounts         []db.ListDashboardStatusCountsRow  `json:"status_counts"`
	CourierCounts        []db.ListDashboardCourierCountsRow `json:"courier_counts"`
	FollowUpsDue         []db.ListDashboardFollowUpsDueRow  `json:"follow_ups_due"`
}

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	now := time.Now().UTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	counts, err := h.Queries.GetDashboardCounts(r.Context(), db.GetDashboardCountsParams{Column1: pgtype.Timestamptz{Time: start, Valid: true}, Column2: pgtype.Timestamptz{Time: end, Valid: true}})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", "failed to load dashboard summary")
		return
	}
	statusCounts, err := h.Queries.ListDashboardStatusCounts(r.Context())
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", "failed to load status counts")
		return
	}
	courierCounts, err := h.Queries.ListDashboardCourierCounts(r.Context(), db.ListDashboardCourierCountsParams{Column1: pgtype.Timestamptz{Time: start, Valid: true}, Column2: pgtype.Timestamptz{Time: end, Valid: true}})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", "failed to load courier counts")
		return
	}
	followUps, err := h.Queries.ListDashboardFollowUpsDue(r.Context(), pgtype.Date{Time: start, Valid: true})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "DASHBOARD_FETCH_FAILED", "failed to load due follow-ups")
		return
	}
	if statusCounts == nil {
		statusCounts = []db.ListDashboardStatusCountsRow{}
	}
	if courierCounts == nil {
		courierCounts = []db.ListDashboardCourierCountsRow{}
	}
	if followUps == nil {
		followUps = []db.ListDashboardFollowUpsDueRow{}
	}
	writeJSON(w, http.StatusOK, dashboardResponse{counts.TodayOrders, counts.PendingConfirmations, counts.ProblemOrders, counts.TotalOrders, statusCounts, courierCounts, followUps})
}

func (h *Handler) CustomerHistory(w http.ResponseWriter, r *http.Request) {
	customerID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_CUSTOMER_ID", "invalid customer id")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id := pgtype.UUID{Bytes: customerID, Valid: true}
	if claims.Role == "staff" {
		orders, fetchErr := h.Queries.ListCustomerOrdersForStaff(r.Context(), id)
		if fetchErr != nil {
			api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_HISTORY_FAILED", "failed to load customer history")
			return
		}
		if orders == nil {
			orders = []db.ListCustomerOrdersForStaffRow{}
		}
		writeJSON(w, http.StatusOK, orders)
		return
	}
	orders, err := h.Queries.ListCustomerOrdersForAdmin(r.Context(), id)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_HISTORY_FAILED", "failed to load customer history")
		return
	}
	if orders == nil {
		orders = []db.Order{}
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) ProblemOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if query.Get("status") == "" {
		query.Set("status", "follow_up")
	}
	r.URL.RawQuery = query.Encode()
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	limit := int32(500)
	if claims.Role == "staff" {
		orders, err := h.Queries.ListProblemOrdersForStaff(r.Context(), limit)
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, "PROBLEM_ORDERS_FAILED", "failed to load problem orders")
			return
		}
		if orders == nil {
			orders = []db.ListProblemOrdersForStaffRow{}
		}
		writeJSON(w, http.StatusOK, orders)
		return
	}
	orders, err := h.Queries.ListProblemOrdersForAdmin(r.Context(), limit)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "PROBLEM_ORDERS_FAILED", "failed to load problem orders")
		return
	}
	if orders == nil {
		orders = []db.Order{}
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) ExportOrders(w http.ResponseWriter, r *http.Request) {
	params, err := parseExportFilters(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_EXPORT_FILTER", "invalid export filter")
		return
	}
	orders, err := h.Queries.ListOrdersForAdmin(r.Context(), params)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "EXPORT_FAILED", "failed to export orders")
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=orders.csv")
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"id", "customer_id", "source", "status", "courier_id", "location_id", "address", "cod_amount", "is_store_visit", "created_by", "created_at", "updated_at", "is_legacy"})
	for _, order := range orders {
		_ = writer.Write([]string{uuidText(order.ID), uuidText(order.CustomerID), string(order.Source), string(order.Status), uuidText(order.CourierID), uuidText(order.LocationID), order.Address, numericText(order.CodAmount), strconv.FormatBool(order.IsStoreVisit), uuidText(order.CreatedBy), order.CreatedAt.Time.Format(time.RFC3339), order.UpdatedAt.Time.Format(time.RFC3339), strconv.FormatBool(order.IsLegacy)})
	}
	writer.Flush()
}

func parseExportFilters(r *http.Request) (db.ListOrdersForAdminParams, error) {
	params := db.ListOrdersForAdminParams{Limit: 10000, Column1: r.URL.Query().Get("search"), Column2: r.URL.Query().Get("status"), Column6: r.URL.Query().Get("source")}
	if value := r.URL.Query().Get("courier_id"); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			return params, err
		}
		params.Column5 = pgtype.UUID{Bytes: id, Valid: true}
	}
	if value := r.URL.Query().Get("customer_id"); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			return params, err
		}
		params.Column7 = pgtype.UUID{Bytes: id, Valid: true}
	}
	for key, target := range map[string]*pgtype.Timestamptz{"from_date": &params.Column3, "to_date": &params.Column4} {
		value := r.URL.Query().Get(key)
		if value == "" {
			continue
		}
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil {
			return params, err
		}
		if key == "to_date" {
			parsed = parsed.Add(24 * time.Hour)
		}
		*target = pgtype.Timestamptz{Time: parsed.UTC(), Valid: true}
	}
	return params, nil
}

func uuidText(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func numericText(value pgtype.Numeric) string {
	if !value.Valid || value.Int == nil {
		return ""
	}
	return value.Int.String()
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
