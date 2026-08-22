package imports

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/legacyimport"

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

func NewHandler(queries *db.Queries, pool *pgxpool.Pool) *Handler {
	return &Handler{Queries: queries, Pool: pool}
}

type startRequest struct {
	Year   int    `json:"year"`
	Source string `json:"source"`
}

type mappedImportRequest struct {
	Year    int               `json:"year"`
	Source  string            `json:"source"`
	Mapping map[string]string `json:"mapping"`
}

type uploadResponse struct {
	ID      string              `json:"id"`
	Status  string              `json:"status"`
	Headers []string            `json:"headers"`
	Preview []map[string]string `json:"preview"`
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	companyID, userID, ok := requestIdentity(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_UPLOAD", "upload is too large or malformed")
		return
	}
	ordersData, ordersName, err := readPart(r, "orders")
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "ORDERS_FILE_REQUIRED", err.Error())
		return
	}
	ordersTable, err := legacyimport.ParseUploadedFile(ordersName, ordersData)
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_ORDERS_FILE", err.Error())
		return
	}
	fileData := map[string][]byte{"orders": ordersData}
	fileNames := map[string]string{"orders": ordersName}
	for _, field := range []string{"products", "couriers", "locations"} {
		data, name, readErr := readPart(r, field)
		if readErr == nil {
			if _, parseErr := legacyimport.ParseUploadedFile(name, data); parseErr != nil {
				api.WriteError(w, http.StatusBadRequest, "INVALID_"+strings.ToUpper(field)+"_FILE", parseErr.Error())
				return
			}
			fileData[field] = data
			fileNames[field] = name
		}
	}
	fileNamesJSON, _ := json.Marshal(fileNames)
	headersJSON, _ := json.Marshal(ordersTable.Headers)
	previewJSON, _ := json.Marshal(ordersTable.Preview)
	upload, err := h.Queries.CreateImportUpload(r.Context(), db.CreateImportUploadParams{
		CompanyID: companyID, UploadedBy: userID, OrdersData: ordersData,
		ProductsData: fileData["products"], CouriersData: fileData["couriers"], LocationsData: fileData["locations"],
		FileNames: fileNamesJSON, Headers: headersJSON, Preview: previewJSON,
	})
	if err != nil {
		api.WriteError(w, http.StatusInternalServerError, "UPLOAD_SAVE_FAILED", "failed to save upload")
		return
	}
	writeJSON(w, http.StatusCreated, uploadView(upload.ID, upload.Status, upload.Headers, upload.Preview))
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	upload, ok := h.getUpload(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, uploadView(upload.ID, upload.Status, upload.Headers, upload.Preview))
}

func (h *Handler) Review(w http.ResponseWriter, r *http.Request) {
	upload, ok := h.getUpload(w, r)
	if !ok {
		return
	}
	if len(upload.ReviewLog) == 0 {
		writeJSON(w, http.StatusOK, []legacyimport.ReviewRow{})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(upload.ReviewLog)
}

func (h *Handler) SaveMappingAndStart(w http.ResponseWriter, r *http.Request) {
	companyID, userID, ok := requestIdentity(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_UPLOAD_ID", "invalid upload id")
		return
	}
	var request mappedImportRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid import request")
		return
	}
	if request.Year < 2000 || request.Year > 2100 || !validSource(request.Source) {
		api.WriteError(w, http.StatusBadRequest, "INVALID_IMPORT_OPTIONS", "invalid year or order source")
		return
	}
	upload, err := h.Queries.GetImportUpload(r.Context(), db.GetImportUploadParams{ID: pgtype.UUID{Bytes: id, Valid: true}, CompanyID: companyID})
	if err != nil {
		writeUploadNotFound(w, err)
		return
	}
	if upload.Status != "mapped" {
		api.WriteError(w, http.StatusConflict, "UPLOAD_NOT_MAPPED", "save a valid mapping before importing")
		return
	}
	run, err := h.Queries.GetLegacyImportRun(r.Context(), companyID)
	if err == nil {
		if run.Status == "failed" {
			run, err = h.Queries.RetryFailedLegacyImportRun(r.Context(), db.RetryFailedLegacyImportRunParams{ID: run.ID, CompanyID: companyID, TriggeredBy: userID})
		} else {
			api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "an import has already started for this company")
			return
		}
	} else if err == pgx.ErrNoRows {
		run, err = h.Queries.CreateLegacyImportRun(r.Context(), db.CreateLegacyImportRunParams{CompanyID: companyID, TriggeredBy: userID})
	}
	if err != nil {
		api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "an import has already started for this company")
		return
	}
	started, err := h.Queries.StartMappedImportUpload(r.Context(), db.StartMappedImportUploadParams{ID: upload.ID, CompanyID: companyID})
	if err != nil {
		api.WriteError(w, http.StatusConflict, "UPLOAD_ALREADY_STARTED", "this upload has already been started")
		return
	}
	go h.runMapped(context.Background(), started, run, companyID, userID, request)
	writeJSON(w, http.StatusAccepted, run)
}

func (h *Handler) SaveMapping(w http.ResponseWriter, r *http.Request) {
	companyID, _, ok := requestIdentity(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_UPLOAD_ID", "invalid upload id")
		return
	}
	upload, err := h.Queries.GetImportUpload(r.Context(), db.GetImportUploadParams{ID: pgtype.UUID{Bytes: id, Valid: true}, CompanyID: companyID})
	if err != nil {
		writeUploadNotFound(w, err)
		return
	}
	var request struct {
		Mapping map[string]string `json:"mapping"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_MAPPING", "invalid mapping")
		return
	}
	var headers []string
	if err := json.Unmarshal(upload.Headers, &headers); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "MAPPING_FAILED", "failed to read upload headers")
		return
	}
	var preview []map[string]string
	if err := json.Unmarshal(upload.Preview, &preview); err != nil {
		api.WriteError(w, http.StatusInternalServerError, "MAPPING_FAILED", "failed to read upload preview")
		return
	}
	rows := legacyimport.UploadedTable{Headers: headers, Preview: preview}
	if _, err := legacyimport.MapUploadedRows(rows, request.Mapping); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_MAPPING", err.Error())
		return
	}
	mappingJSON, _ := json.Marshal(request.Mapping)
	saved, err := h.Queries.SaveImportUploadMapping(r.Context(), db.SaveImportUploadMappingParams{ID: upload.ID, CompanyID: companyID, Mapping: mappingJSON})
	if err != nil {
		api.WriteError(w, http.StatusConflict, "MAPPING_ALREADY_SAVED", "mapping has already been saved")
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (h *Handler) getUpload(w http.ResponseWriter, r *http.Request) (db.ImportUpload, bool) {
	companyID, _, ok := requestIdentity(r)
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return db.ImportUpload{}, false
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_UPLOAD_ID", "invalid upload id")
		return db.ImportUpload{}, false
	}
	upload, err := h.Queries.GetImportUpload(r.Context(), db.GetImportUploadParams{ID: pgtype.UUID{Bytes: id, Valid: true}, CompanyID: companyID})
	if err != nil {
		writeUploadNotFound(w, err)
		return db.ImportUpload{}, false
	}
	return upload, true
}

func readPart(r *http.Request, field string) ([]byte, string, error) {
	file, header, err := r.FormFile(field)
	if err != nil {
		return nil, "", fmt.Errorf("%s file is required", field)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, legacyimport.MaxUploadSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("could not read %s file", field)
	}
	if len(data) > legacyimport.MaxUploadSize {
		return nil, "", fmt.Errorf("%s file exceeds the 25 MB limit", field)
	}
	return data, header.Filename, nil
}

func writeUploadNotFound(w http.ResponseWriter, err error) {
	if err == pgx.ErrNoRows {
		api.WriteError(w, http.StatusNotFound, "UPLOAD_NOT_FOUND", "upload not found")
		return
	}
	api.WriteError(w, http.StatusInternalServerError, "UPLOAD_FETCH_FAILED", "failed to load upload")
}

func uploadView(id pgtype.UUID, status string, headersJSON, previewJSON []byte) uploadResponse {
	var headers []string
	var preview []map[string]string
	_ = json.Unmarshal(headersJSON, &headers)
	_ = json.Unmarshal(previewJSON, &preview)
	return uploadResponse{ID: uuidText(id), Status: status, Headers: headers, Preview: preview}
}

func uuidText(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return uuid.UUID(value.Bytes).String()
}

func requestIdentity(r *http.Request) (pgtype.UUID, pgtype.UUID, bool) {
	companyID, companyOK := auth.GetCompanyID(r.Context())
	claims, claimsOK := auth.GetClaims(r.Context())
	if !companyOK || !claimsOK {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return pgtype.UUID{}, pgtype.UUID{}, false
	}
	return companyID, pgtype.UUID{Bytes: userID, Valid: true}, true
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	var request startRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		api.WriteError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid request")
		return
	}
	if request.Year < 2000 || request.Year > 2100 {
		api.WriteError(w, http.StatusBadRequest, "INVALID_YEAR", "year must be between 2000 and 2100")
		return
	}
	if !validSource(request.Source) {
		api.WriteError(w, http.StatusBadRequest, "INVALID_SOURCE", "invalid order source")
		return
	}
	run, err := h.Queries.GetLegacyImportRun(r.Context(), companyID)
	if err == nil {
		if run.Status != "failed" {
			api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "historical import has already started or completed for this company")
			return
		}
		run, err = h.Queries.RetryFailedLegacyImportRun(r.Context(), db.RetryFailedLegacyImportRunParams{
			ID: run.ID, CompanyID: companyID, TriggeredBy: pgtype.UUID{Bytes: userID, Valid: true},
		})
		if err != nil {
			api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "historical import has already started or completed for this company")
			return
		}
	} else if err != pgx.ErrNoRows {
		api.WriteError(w, http.StatusInternalServerError, "IMPORT_STATUS_FAILED", "failed to check import status")
		return
	} else {
		run, err = h.Queries.CreateLegacyImportRun(r.Context(), db.CreateLegacyImportRunParams{
			CompanyID: companyID, TriggeredBy: pgtype.UUID{Bytes: userID, Valid: true},
		})
		if err != nil {
			api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "historical import has already started or completed for this company")
			return
		}
	}

	go h.run(context.Background(), run.ID, companyID, pgtype.UUID{Bytes: userID, Valid: true}, request)
	writeJSON(w, http.StatusAccepted, run)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	companyID, ok := auth.GetCompanyID(r.Context())
	if !ok {
		api.WriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "unauthorized")
		return
	}
	run, err := h.Queries.GetLegacyImportRun(r.Context(), companyID)
	if err != nil {
		if err == pgx.ErrNoRows {
			api.WriteError(w, http.StatusNotFound, "IMPORT_NOT_FOUND", "historical import has not been started")
			return
		}
		api.WriteError(w, http.StatusInternalServerError, "IMPORT_STATUS_FAILED", "failed to load import status")
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) run(ctx context.Context, runID, companyID, userID pgtype.UUID, request startRequest) {
	if _, err := h.Queries.StartLegacyImportRun(ctx, db.StartLegacyImportRunParams{ID: runID, CompanyID: companyID}); err != nil {
		return
	}
	sourceDir := os.Getenv("LEGACY_SOURCE_DIR")
	cleanup := func() {}
	if sourceDir == "" {
		sourceDir = "legacy-source"
	}
	if _, err := os.Stat(filepath.Join(sourceDir, "orders.csv")); os.IsNotExist(err) && legacyimport.ConfiguredSourceURL() != "" {
		temporaryDir, tempErr := os.MkdirTemp("", "oms-legacy-import-")
		if tempErr == nil {
			sourceDir = temporaryDir
			cleanup = func() { _ = os.RemoveAll(temporaryDir) }
			if tempErr = legacyimport.DownloadSources(sourceDir, legacyimport.ConfiguredSourceURL()); tempErr != nil {
				_ = h.finish(ctx, runID, companyID, "failed", 0, 0, 0, tempErr.Error())
				cleanup()
				return
			}
		}
	}
	defer cleanup()
	importer := &legacyimport.Importer{Pool: h.Pool, Queries: db.New(h.Pool), CreatedBy: userID, CompanyID: companyID}
	report, err := legacyimport.ImportDirectory(ctx, importer, legacyimport.ImportOptions{SourceDir: sourceDir, Year: request.Year, Source: db.OrderSource(request.Source)})
	if err != nil {
		_ = h.finish(ctx, runID, companyID, "failed", 0, 0, 0, err.Error())
		return
	}
	read, inserted, skipped := legacyimport.ImportRunTotals(report)
	_ = h.finish(ctx, runID, companyID, "completed", read, inserted, skipped, "")
}

func (h *Handler) runMapped(ctx context.Context, upload db.ImportUpload, run db.LegacyImportRun, companyID, userID pgtype.UUID, request mappedImportRequest) {
	if _, err := h.Queries.StartLegacyImportRun(ctx, db.StartLegacyImportRunParams{ID: run.ID, CompanyID: companyID}); err != nil {
		return
	}
	var names map[string]string
	if err := json.Unmarshal(upload.FileNames, &names); err != nil {
		_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, nil, err)
		return
	}
	table, err := legacyimport.ParseUploadedFile(names["orders"], upload.OrdersData)
	if err != nil {
		_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, nil, err)
		return
	}
	var mapping map[string]string
	if err := json.Unmarshal(upload.Mapping, &mapping); err != nil {
		_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, nil, err)
		return
	}
	rows, err := legacyimport.MapUploadedRows(table, mapping)
	if err != nil {
		_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, nil, err)
		return
	}
	importer := &legacyimport.Importer{Pool: h.Pool, Queries: db.New(h.Pool), CreatedBy: userID, CompanyID: companyID, Source: db.OrderSource(request.Source), Year: request.Year, AutoCreateProducts: true}
	if len(upload.ProductsData) > 0 {
		productTable, productErr := legacyimport.ParseUploadedFile(names["products"], upload.ProductsData)
		if productErr != nil {
			_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, nil, productErr)
			return
		}
		productReview, productErr := importer.ImportMappedProducts(ctx, productTable)
		importer.Review = append(importer.Review, productReview...)
		if productErr != nil {
			_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, importer.Review, productErr)
			return
		}
	}
	if len(upload.CouriersData) > 0 {
		courierTable, courierErr := legacyimport.ParseUploadedFile(names["couriers"], upload.CouriersData)
		if courierErr == nil {
			courierErr = importer.ImportMappedCouriers(ctx, courierTable)
		}
		if courierErr != nil {
			_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, importer.Review, courierErr)
			return
		}
	}
	if len(upload.LocationsData) > 0 {
		locationTable, locationErr := legacyimport.ParseUploadedFile(names["locations"], upload.LocationsData)
		var locationReview []legacyimport.ReviewRow
		if locationErr == nil {
			locationReview, locationErr = importer.ImportMappedLocations(ctx, locationTable)
			importer.Review = append(importer.Review, locationReview...)
		}
		if locationErr != nil {
			_ = h.finishMapped(ctx, upload, run, companyID, "failed", legacyimport.Counts{}, importer.Review, locationErr)
			return
		}
	}
	counts, err := importer.ImportMapped(ctx, rows)
	if err != nil {
		_ = h.finishMapped(ctx, upload, run, companyID, "failed", counts, importer.Review, err)
		return
	}
	_ = h.finishMapped(ctx, upload, run, companyID, "completed", counts, importer.Review, nil)
}

func (h *Handler) finishMapped(ctx context.Context, upload db.ImportUpload, run db.LegacyImportRun, companyID pgtype.UUID, status string, counts legacyimport.Counts, review []legacyimport.ReviewRow, importErr error) error {
	reviewJSON, _ := json.Marshal(review)
	_, err := h.Queries.CompleteImportUpload(ctx, db.CompleteImportUploadParams{ID: upload.ID, CompanyID: companyID, Status: status, ReviewLog: reviewJSON})
	if err != nil {
		return err
	}
	message := ""
	if importErr != nil {
		message = importErr.Error()
	}
	_, err = h.Queries.CompleteLegacyImportRun(ctx, db.CompleteLegacyImportRunParams{ID: run.ID, CompanyID: companyID, Status: status, RowsRead: int32(counts.Read), RowsInserted: int32(counts.Inserted), RowsSkipped: int32(counts.Skipped), ErrorMessage: pgtype.Text{String: message, Valid: message != ""}})
	return err
}

func (h *Handler) finish(ctx context.Context, runID, companyID pgtype.UUID, status string, read, inserted, skipped int32, message string) error {
	_, err := h.Queries.CompleteLegacyImportRun(ctx, db.CompleteLegacyImportRunParams{
		ID: runID, CompanyID: companyID, Status: status, RowsRead: read, RowsInserted: inserted,
		RowsSkipped: skipped, ErrorMessage: pgtype.Text{String: message, Valid: message != ""},
	})
	return err
}

func validSource(source string) bool {
	switch source {
	case "website", "daraz", "phone", "facebook", "instagram", "store":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
