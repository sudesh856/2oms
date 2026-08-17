package orders

import (
	"encoding/json"
	"net/http"

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
	return &Handler{
		Queries: queries,
	}
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "invalid order id", http.StatusBadRequest)
		return
	}

	id := pgtype.UUID{
		Bytes: orderID,
		Valid: true,
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch claims.Role {
	case "superadmin", "admin":
		order, err := h.Queries.GetOrderForAdmin(r.Context(), id)
		if err != nil {
			if err == pgx.ErrNoRows {
				http.Error(w, "order not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get order", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(order)

	case "staff":
		order, err := h.Queries.GetOrderForStaff(r.Context(), id)
		if err != nil {
			if err == pgx.ErrNoRows {
				http.Error(w, "order not found", http.StatusNotFound)
				return
			}

			http.Error(w, "failed to get order", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(order)

	default:
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch claims.Role {
	case "superadmin", "admin":
		orders, err := h.Queries.ListOrdersForAdmin(r.Context())
		if err != nil {
			http.Error(w, "failed to list orders", http.StatusInternalServerError)
			return
		}

		if orders == nil {
			orders = []db.Order{}
		}

		json.NewEncoder(w).Encode(orders)

	case "staff":
		orders, err := h.Queries.ListOrdersForStaff(r.Context())
		if err != nil {
			http.Error(w, "failed to list orders", http.StatusInternalServerError)
			return
		}

		if orders == nil {
			orders = []db.ListOrdersForStaffRow{}
		}

		json.NewEncoder(w).Encode(orders)

	default:
		http.Error(w, "forbidden", http.StatusForbidden)
	}
}

type createOrderRequest struct {
	CustomerID   string `json:"customer_id"`
	Source       string `json:"source"`
	Address      string `json:"address"`
	CodAmount    string `json:"cod_amount"`
	IsStoreVisit bool   `json:"is_store_visit"`
}

type createStaffOrderRequest struct {
	CustomerID   string `json:"customer_id"`
	Source       string `json:"source"`
	Address      string `json:"address"`
	IsStoreVisit bool   `json:"is_store_visit"`
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	createdBy, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusInternalServerError)
		return
	}

	if claims.Role == "staff" {
		var req createStaffOrderRequest

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		customerID, err := uuid.Parse(req.CustomerID)
		if err != nil {
			http.Error(w, "invalid customer id", http.StatusBadRequest)
			return
		}

		order, err := h.Queries.CreateOrderForStaff(
			r.Context(),
			db.CreateOrderForStaffParams{
				CustomerID: pgtype.UUID{
					Bytes: customerID,
					Valid: true,
				},
				Source:       db.OrderSource(req.Source),
				Status:       db.OrderStatusConfirmed,
				Address:      req.Address,
				IsStoreVisit: req.IsStoreVisit,
				CreatedBy: pgtype.UUID{
					Bytes: createdBy,
					Valid: true,
				},
			},
		)
		if err != nil {
			http.Error(w, "failed to create order", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(order)
		return
	}

	if claims.Role != "admin" && claims.Role != "superadmin" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var req createOrderRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		http.Error(w, "invalid customer id", http.StatusBadRequest)
		return
	}

	var codAmount pgtype.Numeric
	if err := codAmount.Scan(req.CodAmount); err != nil {
		http.Error(w, "invalid cod amount", http.StatusBadRequest)
		return
	}

	order, err := h.Queries.CreateOrderForAdmin(
		r.Context(),
		db.CreateOrderForAdminParams{
			CustomerID: pgtype.UUID{
				Bytes: customerID,
				Valid: true,
			},
			Source:       db.OrderSource(req.Source),
			Status:       db.OrderStatusConfirmed,
			Address:      req.Address,
			CodAmount:    codAmount,
			IsStoreVisit: req.IsStoreVisit,
			CreatedBy: pgtype.UUID{
				Bytes: createdBy,
				Valid: true,
			},
		},
	)
	if err != nil {
		http.Error(w, "failed to create order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	json.NewEncoder(w).Encode(order)
}
