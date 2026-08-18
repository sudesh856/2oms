package customers

import (
	"encoding/json"
	"net/http"
	"strings"

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
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Phone = strings.TrimSpace(req.Phone)
	req.Name = strings.TrimSpace(req.Name)

	if req.Phone == "" {
		http.Error(w, "phone is required", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	existing, err := h.Queries.GetCustomerByPhone(r.Context(), req.Phone)
	if err == nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}

	if err != pgx.ErrNoRows {
		http.Error(w, "failed to check customer", http.StatusInternalServerError)
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
		http.Error(w, "failed to create customer", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) SearchByPhone(w http.ResponseWriter, r *http.Request) {
	phone := strings.TrimSpace(r.URL.Query().Get("phone"))

	if phone == "" {
		http.Error(w, "phone is required", http.StatusBadRequest)
		return
	}

	customer, err := h.Queries.GetCustomerByPhone(
		r.Context(),
		phone,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "customer not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to search customer", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, customer)
}

func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var customerID pgtype.UUID

	if err := customerID.Scan(id); err != nil {
		http.Error(w, "invalid customer id", http.StatusBadRequest)
		return
	}

	customer, err := h.Queries.GetCustomerByID(
		r.Context(),
		customerID,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "customer not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get customer", http.StatusInternalServerError)
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
		http.Error(w, "failed to list customers", http.StatusInternalServerError)
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
