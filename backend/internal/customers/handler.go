package customers

import (
	"encoding/json"
	"net/http"
	"strings"

	"oms-backend/internal/api"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
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

type createCustomerRequest struct {
	Phone   string  `json:"phone"`
	Phone2  *string `json:"phone2"`
	Name    string  `json:"name"`
	Address *string `json:"address"`
}

func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req createCustomerRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.Name = strings.TrimSpace(req.Name)

	if req.Phone == "" {
		api.WriteError(w, http.StatusBadRequest, "PHONE_REQUIRED", "phone is required")
		return
	}

	if req.Name == "" {
		api.WriteError(w, http.StatusBadRequest, "NAME_REQUIRED", "name is required")
		return
	}

	existing, err := h.Queries.GetCustomerByPhone(r.Context(), req.Phone)
	if err == nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	if err != pgx.ErrNoRows {
		api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_CHECK_FAILED", "failed to check customer")
		return
	}

	customer, err := h.Queries.CreateCustomer(
		r.Context(),
		db.CreateCustomerParams{
			Phone:   req.Phone,
			Phone2:  textToNullable(req.Phone2),
			Name:    req.Name,
			Address: textToNullable(req.Address),
		},
	)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_CREATE_FAILED", "failed to create customer")
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) SearchByPhone(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))

	if phone == "" {
		api.WriteError(w, http.StatusBadRequest, "PHONE_REQUIRED", "phone is required")
		return
	}

	customer, err := h.Queries.GetCustomerByPhone(
		r.Context(),
		phone,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
			return
		}

		api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_SEARCH_FAILED", "failed to search customer")
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var customerID pgtype.UUID

	if err := customerID.Scan(id); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_CUSTOMER_ID", "invalid customer id")
		return
	}

	customer, err := h.Queries.GetCustomerByID(
		r.Context(),
		customerID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusNotFound, "CUSTOMER_NOT_FOUND", "customer not found")
			return
		}

		api.WriteError(w, http.StatusInternalServerError, "CUSTOMER_FETCH_FAILED", "failed to get customer")
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	customers, err := h.Queries.ListCustomers(
		r.Context(),
		db.ListCustomersParams{
			Column1: search,
			Limit:   50,
			Offset:  0,
		},
	)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "CUSTOMERS_FETCH_FAILED", "failed to list customers")
		return
	}

	if customers == nil {
		customers = []db.Customer{}
	}

	writeJSON(w, http.StatusOK, customers)
}

func textToNullable(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{
		String: *value,
		Valid:  true,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
