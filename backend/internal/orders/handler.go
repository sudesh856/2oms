package orders

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		"CodAmount":    "cod_amount",
		"CODAmount":    "cod_amount",
		"IsStoreVisit": "is_store_visit",
		"CreatedBy":    "created_by",
		"CreatedAt":    "created_at",
		"UpdatedAt":    "updated_at",
		"IsLegacy":     "is_legacy",
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

func writeCreatedOrderJSON(w http.ResponseWriter, order any, warnings []StockWarning, staff bool) {
	data, err := json.Marshal(order)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "ORDER_SERIALIZE_FAILED", "failed to serialize order")
		return
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "ORDER_SERIALIZE_FAILED", "failed to serialize order")
		return
	}

	rename := map[string]string{
		"ID": "id", "CustomerID": "customer_id", "Source": "source", "Status": "status",
		"CourierID": "courier_id", "LocationID": "location_id", "Address": "address",
		"CodAmount": "cod_amount", "CODAmount": "cod_amount", "IsStoreVisit": "is_store_visit", "CreatedBy": "created_by",
		"CreatedAt": "created_at", "UpdatedAt": "updated_at", "IsLegacy": "is_legacy",
	}
	result := make(map[string]any, len(raw)+1)
	for key, value := range raw {
		jsonKey := rename[key]
		if jsonKey == "" {
			jsonKey = key
		}
		if staff && jsonKey == "cod_amount" {
			continue
		}
		result[jsonKey] = value
	}

	stockWarnings := make([]map[string]any, 0, len(warnings))
	for _, warning := range warnings {
		stockWarnings = append(stockWarnings, map[string]any{
			"product_id":    warning.ProductID,
			"product_name":  warning.ProductName,
			"requested_qty": warning.RequestedQty,
			"available_qty": warning.AvailableQty,
		})
	}
	result["stock_warnings"] = stockWarnings

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
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
		"CodAmount":    "cod_amount",
		"CODAmount":    "cod_amount",
		"IsStoreVisit": "is_store_visit",
		"CreatedBy":    "created_by",
		"CreatedAt":    "created_at",
		"UpdatedAt":    "updated_at",
		"IsLegacy":     "is_legacy",
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

func writeOrderItemsJSON(w http.ResponseWriter, items any) {
	data, err := json.Marshal(items)
	if err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_ITEMS_SERIALIZE_FAILED",
			"failed to serialize order items",
		)
		return
	}

	var raw []map[string]any

	if err := json.Unmarshal(data, &raw); err != nil {
		api.WriteError(
			w,
			http.StatusInternalServerError,
			"ORDER_ITEMS_SERIALIZE_FAILED",
			"failed to serialize order items",
		)
		return
	}

	rename := map[string]string{
		"ID":        "id",
		"OrderID":   "order_id",
		"ProductID": "product_id",
		"Quantity":  "quantity",
		"Price":     "price",
	}

	result := make([]map[string]any, 0, len(raw))

	for _, item := range raw {
		normalized := make(map[string]any, len(item))

		for key, value := range item {
			jsonKey, ok := rename[key]

			if !ok {
				jsonKey = key
			}

			normalized[jsonKey] = value
		}

		result = append(result, normalized)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) GetOrderItems(w http.ResponseWriter, r *http.Request) {
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
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
		if _, err := h.Queries.GetOrderForAdmin(r.Context(), db.GetOrderForAdminParams{ID: id, CompanyID: companyID}); err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}
			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}
	case "staff":
		if _, err := h.Queries.GetOrderForStaff(r.Context(), db.GetOrderForStaffParams{ID: id, CompanyID: companyID}); err != nil {
			if err == pgx.ErrNoRows {
				api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
				return
			}
			api.WriteError(w, http.StatusInternalServerError, "ORDER_FETCH_FAILED", "failed to get order")
			return
		}
	default:
		api.WriteError(w, http.StatusForbidden, "FORBIDDEN", "forbidden")
		return
	}

	items, err := h.Queries.ListOrderItems(r.Context(), db.ListOrderItemsParams{
		OrderID:   id,
		CompanyID: companyID,
	})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "ORDER_ITEMS_FETCH_FAILED", "failed to list order items")
		return
	}

	if items == nil {
		items = []db.ListOrderItemsRow{}
	}

	writeOrderItemsJSON(w, items)
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
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
		order, err := h.Queries.GetOrderForAdmin(r.Context(), db.GetOrderForAdminParams{ID: id, CompanyID: companyID})
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

		order, err := h.Queries.GetOrderForStaff(r.Context(), db.GetOrderForStaffParams{ID: id, CompanyID: companyID})
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
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	params, err := parseOrderFilters(r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDER_FILTER", "invalid order filter")
		return
	}
	params.Column10 = companyID

	w.Header().Set("Content-Type", "application/json")

	switch claims.Role {
	case "superadmin", "admin":
		orders, err := h.Queries.ListOrdersForAdmin(r.Context(), params)
		if err != nil {
			api.WriteError(w, http.StatusInternalServerError, "ORDERS_FETCH_FAILED", "failed to list orders")
			return
		}

		if orders == nil {
			orders = []db.ListOrdersForAdminRow{}
		}

		writeOrdersJSON(w, orders, false)

	case "staff":
		staffParams := db.ListOrdersForStaffParams{
			Column1: params.Column1, Column2: params.Column2, Column3: params.Column3,
			Column4: params.Column4, Column5: params.Column5, Column6: params.Column6,
			Column7: params.Column7, Offset: params.Offset, Limit: params.Limit, Column10: companyID,
		}
		orders, err := h.Queries.ListOrdersForStaff(r.Context(), staffParams)
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

func parseOrderFilters(r *http.Request) (db.ListOrdersForAdminParams, error) {
	params := db.ListOrdersForAdminParams{Limit: 100}
	params.Column1 = strings.TrimSpace(r.URL.Query().Get("search"))
	params.Column2 = strings.TrimSpace(r.URL.Query().Get("status"))
	params.Column6 = strings.TrimSpace(r.URL.Query().Get("source"))
	if value := r.URL.Query().Get("limit"); value != "" {
		limit, err := strconv.Atoi(value)
		if err != nil || limit < 1 || limit > 500 {
			return params, fmt.Errorf("invalid limit")
		}
		params.Limit = int32(limit)
	}
	if value := r.URL.Query().Get("offset"); value != "" {
		offset, err := strconv.Atoi(value)
		if err != nil || offset < 0 {
			return params, fmt.Errorf("invalid offset")
		}
		params.Offset = int32(offset)
	}
	for key, target := range map[string]*pgtype.Timestamptz{"from_date": &params.Column3, "to_date": &params.Column4} {
		value := strings.TrimSpace(r.URL.Query().Get(key))
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
	if value := strings.TrimSpace(r.URL.Query().Get("courier_id")); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return params, err
		}
		params.Column5 = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	if value := strings.TrimSpace(r.URL.Query().Get("customer_id")); value != "" {
		parsed, err := uuid.Parse(value)
		if err != nil {
			return params, err
		}
		params.Column7 = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	return params, nil
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
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
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
		order, err := h.Queries.GetOrderForAdmin(r.Context(), db.GetOrderForAdminParams{ID: orderIDPG, CompanyID: companyID})
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
		order, err := h.Queries.GetOrderForStaff(r.Context(), db.GetOrderForStaffParams{ID: orderIDPG, CompanyID: companyID})
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
		companyID,
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
		order, err := h.Queries.GetOrderForAdmin(r.Context(), db.GetOrderForAdminParams{ID: orderIDPG, CompanyID: companyID})
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
		order, err := h.Queries.GetOrderForStaff(r.Context(), db.GetOrderForStaffParams{ID: orderIDPG, CompanyID: companyID})
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
	CourierID    *string                  `json:"courier_id"`
	LocationID   *string                  `json:"location_id"`
	Items        []createOrderItemRequest `json:"items"`
}

type createStaffOrderRequest struct {
	CustomerID   string                   `json:"customer_id"`
	Source       string                   `json:"source"`
	Address      string                   `json:"address"`
	IsStoreVisit bool                     `json:"is_store_visit"`
	CourierID    *string                  `json:"courier_id"`
	LocationID   *string                  `json:"location_id"`
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
	companyID, companyOK := auth.GetCompanyID(r.Context())
	if !companyOK {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
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
			CourierID:    staffReq.CourierID,
			LocationID:   staffReq.LocationID,
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

	var courierID pgtype.UUID
	if req.CourierID != nil && strings.TrimSpace(*req.CourierID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.CourierID))
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
			return
		}
		courierID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	var locationID pgtype.UUID
	if req.LocationID != nil && strings.TrimSpace(*req.LocationID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.LocationID))
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_LOCATION_ID", "invalid location id")
			return
		}
		locationID = pgtype.UUID{Bytes: parsed, Valid: true}
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

	order, warnings, err := service.CreateOrderWithItemsAndWarnings(r.Context(),
		CreateOrderInput{
			CustomerID:   customerID,
			Source:       db.OrderSource(req.Source),
			Address:      req.Address,
			CodAmount:    codAmount,
			IsStoreVisit: req.IsStoreVisit,
			CourierID:    courierID,
			LocationID:   locationID,
			CreatedBy:    createdBy,
			CompanyID:    uuid.UUID(companyID.Bytes),
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

	writeCreatedOrderJSON(w, order, warnings, claims.Role == "staff")
}

type updateOrderRequest struct {
	CourierID  *string `json:"courier_id"`
	LocationID *string `json:"location_id"`
}

func (h *Handler) UpdateOrder(w http.ResponseWriter, r *http.Request) {
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

	orderID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDER_ID", "invalid order id")
		return
	}

	orderIDPG := pgtype.UUID{
		Bytes: orderID,
		Valid: true,
	}

	var req updateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}

	var courierID pgtype.UUID
	if req.CourierID != nil && strings.TrimSpace(*req.CourierID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.CourierID))
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
			return
		}
		courierID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	var locationID pgtype.UUID
	if req.LocationID != nil && strings.TrimSpace(*req.LocationID) != "" {
		parsed, err := uuid.Parse(strings.TrimSpace(*req.LocationID))
		if err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_LOCATION_ID", "invalid location id")
			return
		}
		locationID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	order, err := h.Queries.UpdateOrderCourierAndLocation(
		r.Context(),
		db.UpdateOrderCourierAndLocationParams{
			ID:         orderIDPG,
			CourierID:  courierID,
			LocationID: locationID,
			CompanyID:  companyID,
		},
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusNotFound, "ORDER_NOT_FOUND", "order not found")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "ORDER_UPDATE_FAILED", "failed to update order")
		return
	}

	writeOrderJSON(w, order, claims.Role == "staff", http.StatusOK)
}

