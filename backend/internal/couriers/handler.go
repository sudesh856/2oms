package couriers

import (
	"encoding/json"
	"net/http"
	"strings"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	Queries *db.Queries
}

func NewHandler(queries *db.Queries) *Handler { return &Handler{Queries: queries} }

type courierRequest struct {
	Name string `json:"name"`
}
type locationRequest struct {
	LocationName   string `json:"location_name"`
	DeliveryCharge string `json:"delivery_charge"`
}

func (h *Handler) ListCouriers(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	items, err := h.Queries.ListCouriers(r.Context(), companyID)
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "COURIERS_FETCH_FAILED", "failed to list couriers")
		return
	}
	if items == nil {
		items = []db.ListCouriersRow{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateCourier(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	var req courierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		api.WriteError(w, http.StatusBadRequest, "NAME_REQUIRED", "name is required")
		return
	}
	item, err := h.Queries.CreateCourier(r.Context(), db.CreateCourierParams{Name: req.Name, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "COURIER_CREATE_FAILED", "failed to create courier")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) UpdateCourier(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id, err := parseUUIDParam("id", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	var req courierRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		api.WriteError(w, http.StatusBadRequest, "NAME_REQUIRED", "name is required")
		return
	}
	item, err := h.Queries.UpdateCourier(r.Context(), db.UpdateCourierParams{ID: id, CompanyID: companyID, Name: req.Name})
	if err != nil {
		writeNotFoundOrError(w, err, "COURIER_UPDATE_FAILED", "failed to update courier")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteCourier(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id, err := parseUUIDParam("id", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	result, err := h.Queries.DeleteCourier(r.Context(), db.DeleteCourierParams{ID: id, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusConflict, "COURIER_DELETE_FAILED", "failed to delete courier")
		return
	}
	if result.RowsAffected() == 0 {
		api.WriteError(w, http.StatusNotFound, "COURIER_NOT_FOUND", "courier not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	courierID, err := parseUUIDParam("courierID", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	items, err := h.Queries.ListCourierLocations(r.Context(), db.ListCourierLocationsParams{CourierID: courierID, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "LOCATIONS_FETCH_FAILED", "failed to list courier locations")
		return
	}
	if items == nil {
		items = []db.ListCourierLocationsRow{}
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	courierID, err := parseUUIDParam("courierID", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	var req locationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.LocationName = strings.TrimSpace(req.LocationName)
	if req.LocationName == "" {
		api.WriteError(w, http.StatusBadRequest, "LOCATION_REQUIRED", "location name is required")
		return
	}
	charge := pgtype.Numeric{}
	if req.DeliveryCharge != "" {
		if err := charge.Scan(req.DeliveryCharge); err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_DELIVERY_CHARGE", "invalid delivery charge")
			return
		}
	}
	item, err := h.Queries.CreateCourierLocation(r.Context(), db.CreateCourierLocationParams{CourierID: courierID, CompanyID: companyID, LocationName: req.LocationName, DeliveryCharge: charge})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "LOCATION_CREATE_FAILED", "failed to create courier location")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	courierID, err := parseUUIDParam("courierID", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	id, err := parseUUIDParam("id", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_LOCATION_ID", "invalid location id")
		return
	}
	var req locationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	req.LocationName = strings.TrimSpace(req.LocationName)
	if req.LocationName == "" {
		api.WriteError(w, http.StatusBadRequest, "LOCATION_REQUIRED", "location name is required")
		return
	}
	charge := pgtype.Numeric{}
	if req.DeliveryCharge != "" {
		if err := charge.Scan(req.DeliveryCharge); err != nil {
			api.WriteError(w, http.StatusBadRequest, "INVALID_DELIVERY_CHARGE", "invalid delivery charge")
			return
		}
	}
	item, err := h.Queries.UpdateCourierLocation(r.Context(), db.UpdateCourierLocationParams{ID: id, CourierID: courierID, CompanyID: companyID, LocationName: req.LocationName, DeliveryCharge: charge})
	if err != nil {
		writeNotFoundOrError(w, err, "LOCATION_UPDATE_FAILED", "failed to update courier location")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h *Handler) DeleteLocation(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	courierID, err := parseUUIDParam("courierID", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_COURIER_ID", "invalid courier id")
		return
	}
	id, err := parseUUIDParam("id", r)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_LOCATION_ID", "invalid location id")
		return
	}
	result, err := h.Queries.DeleteCourierLocation(r.Context(), db.DeleteCourierLocationParams{ID: id, CourierID: courierID, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "LOCATION_DELETE_FAILED", "failed to delete courier location")
		return
	}
	if result.RowsAffected() == 0 {
		api.WriteError(w, http.StatusNotFound, "LOCATION_NOT_FOUND", "location not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseUUIDParam(name string, r *http.Request) (pgtype.UUID, error) {
	parsed := pgtype.UUID{}
	if err := parsed.Scan(chi.URLParam(r, name)); err != nil {
		return pgtype.UUID{}, err
	}
	return parsed, nil
}

func writeNotFoundOrError(w http.ResponseWriter, err error, code, message string) {
	if err == pgx.ErrNoRows {
		api.WriteError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
		return
	}
	api.WriteError(w, http.StatusInternalServerError, code, message)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
