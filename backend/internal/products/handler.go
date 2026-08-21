package products

import (
	"encoding/json"
	"net/http"
	"strings"

	"oms-backend/internal/auth"
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

type createProductRequest struct {
	Name         string `json:"name"`
	Price        string `json:"price"`
	AvailableQty int32  `json:"available_qty"`
	WarehouseQty int32  `json:"warehouse_qty"`
}

type updateProductRequest struct {
	Name         string `json:"name"`
	Price        string `json:"price"`
	AvailableQty int32  `json:"available_qty"`
	WarehouseQty int32  `json:"warehouse_qty"`
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req createProductRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if req.AvailableQty < 0 {
		http.Error(w, "available_qty cannot be negative", http.StatusBadRequest)
		return
	}

	if req.WarehouseQty < 0 {
		http.Error(w, "warehouse_qty cannot be negative", http.StatusBadRequest)
		return
	}

	var price pgtype.Numeric

	if err := price.Scan(req.Price); err != nil {
		http.Error(w, "invalid price", http.StatusBadRequest)
		return
	}

	product, err := h.Queries.CreateProduct(
		r.Context(),
		db.CreateProductParams{
			Name:         req.Name,
			Price:        price,
			AvailableQty: req.AvailableQty,
			WarehouseQty: req.WarehouseQty,
			CompanyID:    companyID,
		},
	)
	if err != nil {
		http.Error(w, "failed to create product", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")

	var productID pgtype.UUID

	if err := productID.Scan(id); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	product, err := h.Queries.GetProductByID(
		r.Context(),
		db.GetProductByIDParams{ID: productID, CompanyID: companyID},
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to get product", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	search := strings.TrimSpace(r.URL.Query().Get("search"))

	products, err := h.Queries.ListProducts(
		r.Context(),
		db.ListProductsParams{CompanyID: companyID, Column2: search},
	)
	if err != nil {
		http.Error(w, "failed to list products", http.StatusInternalServerError)
		return
	}

	if products == nil {
		products = []db.ListProductsRow{}
	}

	writeJSON(w, http.StatusOK, products)
}

func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := chi.URLParam(r, "id")

	var productID pgtype.UUID

	if err := productID.Scan(id); err != nil {
		http.Error(w, "invalid product id", http.StatusBadRequest)
		return
	}

	var req updateProductRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	if req.AvailableQty < 0 {
		http.Error(w, "available_qty cannot be negative", http.StatusBadRequest)
		return
	}

	if req.WarehouseQty < 0 {
		http.Error(w, "warehouse_qty cannot be negative", http.StatusBadRequest)
		return
	}

	var price pgtype.Numeric

	if err := price.Scan(req.Price); err != nil {
		http.Error(w, "invalid price", http.StatusBadRequest)
		return
	}

	product, err := h.Queries.UpdateProduct(
		r.Context(),
		db.UpdateProductParams{
			ID:           productID,
			Name:         req.Name,
			Price:        price,
			AvailableQty: req.AvailableQty,
			WarehouseQty: req.WarehouseQty,
			CompanyID:    companyID,
		},
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}

		http.Error(w, "failed to update product", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, product)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(value)
}
