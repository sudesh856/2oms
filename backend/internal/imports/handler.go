package imports

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"oms-backend/internal/api"
	"oms-backend/internal/auth"
	db "oms-backend/internal/db/generated"
	"oms-backend/internal/legacyimport"

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
	if _, err := h.Queries.GetLegacyImportRun(r.Context(), companyID); err == nil {
		api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "historical import has already been started for this company")
		return
	} else if err != pgx.ErrNoRows {
		api.WriteError(w, http.StatusInternalServerError, "IMPORT_STATUS_FAILED", "failed to check import status")
		return
	}
	run, err := h.Queries.CreateLegacyImportRun(r.Context(), db.CreateLegacyImportRunParams{
		CompanyID: companyID, TriggeredBy: pgtype.UUID{Bytes: userID, Valid: true},
	})
	if err != nil {
		api.WriteError(w, http.StatusConflict, "IMPORT_ALREADY_EXISTS", "historical import has already been started for this company")
		return
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
