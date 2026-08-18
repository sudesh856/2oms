package orders

import (
	"encoding/json"
	"net/http"
	"strings"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	Queries *db.Queries
	Pool    *pgxpool.Pool
}
type createOrderItemRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

func NewHandler(queries *db.Queries, pool *pgxpool.Pool) *Handler {
	return &Handler{
		Queries: queries,
		Pool:    pool,
	}
}

func writeOrderJSON(w http.ResponseWriter, order any, staff bool, statusCode int) {
	data, err := json.Marshal(order)
	if err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_SERIALIZE_FAILED",
			"failed to serialize order",
		)
		return
	}

	var raw map[string]any

	if err := json.Unmarshal(data, &raw); err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_SERIALIZE_FAILED",
			"failed to serialize order",
		)
		return
	}

	// Convert Go/SQLC field names to the JSON contract
	rename := map[string]string{
		"ID":           "id",
		"CustomerID":   "customer_id",
		"Source":       "source",
		"Status":       "status",
		"CourierID":    "courier_id",
		"LocationID":   "location_id",
		"Address":      "address",
		"CODAmount":    "cod_amount",
		"IsStoreVisit": "is_store_visit",
		"CreatedBy":    "created_by",
		"CreatedAt":    "created_at",
		"UpdatedAt":    "updated_at",
	}

	result := make(map[string]any, len(raw))

	for key, value := range raw {
		jsonKey, ok := rename[key]

		if !ok {
			jsonKey = key
		}

		if staff && jsonKey == "cod_amount" {
			continue
		}

		result[jsonKey] = value
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(result)
}

func writeOrdersJSON(w http.ResponseWriter, orders any, staff bool) {
	data, err := json.Marshal(orders)
	if err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDERS_SERIALIZE_FAILED",
			"failed to serialize orders",
		)
		return
	}

	var raw []map[string]any

	if err := json.Unmarshal(data, &raw); err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDERS_SERIALIZE_FAILED",
			"failed to serialize orders",
		)
		return
	}

	rename := map[string]string{
		"ID":           "id",
		"CustomerID":   "customer_id",
		"Source":       "source",
		"Status":       "status",
		"CourierID":    "courier_id",
		"LocationID":   "location_id",
		"Address":      "address",
		"CODAmount":    "cod_amount",
		"IsStoreVisit": "is_store_visit",
		"CreatedBy":    "created_by",
		"CreatedAt":    "created_at",
		"UpdatedAt":    "updated_at",
	}

	result := make([]map[string]any, 0, len(raw))

	for _, item := range raw {
		normalized := make(map[string]any, len(item))

		for key, value := range item {
			jsonKey, ok := rename[key]

			if !ok {
				jsonKey = key
			}

			if staff && jsonKey == "cod_amount" {
				continue
			}

			normalized[jsonKey] = value
		}

		result = append(result, normalized)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	id := pgtype.UUID{
		Bytes: orderID,
		Valid: true,
	}

	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	switch claims.Role {
	case "superadmin", "admin":
		order, err := h.Queries.GetOrderForAdmin(r.Context(), id)
		if err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}

			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}

		writeOrderJSON(w, order, false, http.StatusOK)

	case "staff":

		order, err := h.Queries.GetOrderForStaff(r.Context(), id)
		if err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}

			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}

		writeOrderJSON(w, order, true, http.StatusOK)

	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
	}
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch claims.Role {
	case "superadmin", "admin":
		orders, err := h.Queries.ListOrdersForAdmin(r.Context())
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, "ORDERS_FETCH_FAILED", "failed to list orders")
			return
		}

		if orders == nil {
			orders = []db.Order{}
		}

		writeOrdersJSON(w, orders, false)

	case "staff":
		orders, err := h.Queries.ListOrdersForStaff(r.Context())
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, "ORDERS_FETCH_FAILED", "failed to list orders")
			return
		}

		if orders == nil {
			orders = []db.ListOrdersForStaffRow{}
		}

		writeOrdersJSON(w, orders, true)

	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
	}
}

type updateOrderStatusRequest struct {
	Status string `json:"status"`
}

func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	var req updateOrderStatusRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	req.Status = strings.TrimSpace(req.Status)

	if req.Status == "" {
		api.WriteError(w, http.StatusBadRequest, "INVALID_STATUS", "status is required")
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVALID_USER_ID", "invalid user id")
		return
	}

	orderIDPG := pgtype.UUID{
		Bytes: orderID,
		Valid: true,
	}

	var fromStatus db.OrderStatus

	switch claims.Role {
	case "superadmin", "admin":
		order, err := h.Queries.GetOrderForAdmin(r.Context(), orderIDPG)
		if err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}

			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}

		fromStatus = order.Status

	case "staff":
		order, err := h.Queries.GetOrderForStaff(r.Context(), orderIDPG)
		if err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}

			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}

		fromStatus = order.Status

	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}
	toStatus := db.OrderStatus(req.Status)

	if err := ValidateTransition(string(fromStatus), string(toStatus)); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_STATUS_TRANSITION", err.Error())
		return
	}

	service := NewService(h.Pool)

	if err := service.UpdateStatus(
		r.Context(),
		orderIDPG,
		fromStatus,
		toStatus,
		pgtype.UUID{
			Bytes: userID,
			Valid: true,
		},
	); err != nil {
		if strings.Contains(err.Error(), "order not found") {
			api.WriteError(
				w,
				http.StatusConflict,
				"ORDER_STATUS_CHANGED",
				"order status changed; please refresh and try again",
			)
			return
		}

		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_STATUS_UPDATE_FAILED",
			"failed to update order status",
		)
		return
	}

	var updatedOrder any

	switch claims.Role {
	case "superadmin", "admin":
		order, err := h.Queries.GetOrderForAdmin(r.Context(), orderIDPG)
		if err != nil {
			api.WriteError(
				w,
				http.StatusInternalServerError,
				"ORDER_FETCH_FAILED",
				"failed to get updated order",
			)
			return
		}
		updatedOrder = order

	case "staff":
		order, err := h.Queries.GetOrderForStaff(r.Context(), orderIDPG)
		if err != nil {
			api.WriteError(
				w,
				http.StatusInternalServerError,
				"ORDER_FETCH_FAILED",
				"failed to get updated order",
			)
			return
		}
		updatedOrder = order

	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}

	writeOrderJSON(w, updatedOrder, claims.Role == "staff", http.StatusOK)
}

type createOrderRequest struct {
	CustomerID   string                   `json:"customer_id"`
	Source       string                   `json:"source"`
	Address      string                   `json:"address"`
	CodAmount    string                   `json:"cod_amount"`
	IsStoreVisit bool                     `json:"is_store_visit"`
	Items        []createOrderItemRequest `json:"items"`
}

type createStaffOrderRequest struct {
	CustomerID   string                   `json:"customer_id"`
	Source       string                   `json:"source"`
	Address      string                   `json:"address"`
	IsStoreVisit bool                     `json:"is_store_visit"`
	Items        []createOrderItemRequest `json:"items"`
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}

	createdBy, err := uuid.Parse(claims.UserID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "INVALID_USER_ID", "invalid user id")
		return
	}

	var req createOrderRequest

	if claims.Role == "staff" {
		var staffReq createStaffOrderRequest

		if err := json.NewDecoder(r.Body).Decode(&staffReq); err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
			return
		}

		req = createOrderRequest{
			CustomerID:   staffReq.CustomerID,
			Source:       staffReq.Source,
			Address:      staffReq.Address,
			IsStoreVisit: staffReq.IsStoreVisit,
			Items:        staffReq.Items,
		}
	} else if claims.Role == "admin" || claims.Role == "superadmin" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
			return
		}
	} else {
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}

	customerID, err := uuid.Parse(req.CustomerID)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_CUSTOMER_ID", "invalid customer id")
		return
	}

	var codAmount pgtype.Numeric

	if claims.Role == "admin" || claims.Role == "superadmin" {
		if err := codAmount.Scan(req.CodAmount); err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_COD_AMOUNT", "invalid cod amount")
			return
		}
	}
	items := make([]CreateOrderItem, 0, len(req.Items))

	for _, item := range req.Items {
		productID, err := uuid.Parse(item.ProductID)
		if err != nil {
			api.WriteError(
				w,
				http.StatusBadRequest,
				"INVALID_PRODUCT_ID",
				"invalid product id",
			)
			return
		}

		if item.Quantity <= 0 {
			api.WriteError(
				w,
				http.StatusBadRequest,
				"INVALID_QUANTITY",
				"quantity must be greater than zero",
			)
			return
		}

		items = append(items, CreateOrderItem{
			ProductID: productID,
			Quantity:  item.Quantity,
		})
	}

	service := &Service{
		Pool: h.Pool,
	}

	order, err := service.CreateOrderWithItems(r.Context(),
		CreateOrderInput{
			CustomerID:   customerID,
			Source:       db.OrderSource(req.Source),
			Address:      req.Address,
			CodAmount:    codAmount,
			IsStoreVisit: req.IsStoreVisit,
			CreatedBy:    createdBy,
			Items:        items,
		},
		claims.Role == "staff",
	)
	if err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_CREATE_FAILED",
			err.Error(),
		)
		return
	}

	writeOrderJSON(w, order, claims.Role == "staff", http.StatusCreated)
}
