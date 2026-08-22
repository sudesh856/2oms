-- name: CreateLegacyImportRun :one
INSERT INTO legacy_import_runs (company_id, triggered_by, status)
VALUES ($1, $2, 'queued')
RETURNING id, company_id, triggered_by, status, rows_read, rows_inserted, rows_skipped,
          error_message, created_at, started_at, completed_at;

-- name: GetLegacyImportRun :one
SELECT id, company_id, triggered_by, status, rows_read, rows_inserted, rows_skipped,
       error_message, created_at, started_at, completed_at
FROM legacy_import_runs
WHERE company_id = $1;

-- name: StartLegacyImportRun :one
UPDATE legacy_import_runs
SET status = 'running', started_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'queued'
RETURNING id, company_id, triggered_by, status, rows_read, rows_inserted, rows_skipped,
          error_message, created_at, started_at, completed_at;

-- name: CompleteLegacyImportRun :one
UPDATE legacy_import_runs
SET status = $3, rows_read = $4, rows_inserted = $5, rows_skipped = $6,
    error_message = $7, completed_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING id, company_id, triggered_by, status, rows_read, rows_inserted, rows_skipped,
          error_message, created_at, started_at, completed_at;

-- name: FailInterruptedLegacyImportRuns :exec
UPDATE legacy_import_runs
SET status = 'failed', error_message = 'import interrupted by server restart', completed_at = NOW()
WHERE status IN ('queued', 'running');

-- name: RetryFailedLegacyImportRun :one
UPDATE legacy_import_runs
SET status = 'queued', triggered_by = $3, rows_read = 0, rows_inserted = 0,
    rows_skipped = 0, error_message = NULL, started_at = NULL, completed_at = NULL
WHERE id = $1 AND company_id = $2 AND status = 'failed'
RETURNING id, company_id, triggered_by, status, rows_read, rows_inserted, rows_skipped,
          error_message, created_at, started_at, completed_at;