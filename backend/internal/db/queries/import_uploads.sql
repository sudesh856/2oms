-- name: CreateImportUpload :one
INSERT INTO import_uploads (company_id, uploaded_by, orders_data, products_data, couriers_data, locations_data, file_names, headers, preview)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, company_id, uploaded_by, file_names, headers, preview, mapping, review_log, status, created_at, updated_at;

-- name: GetImportUpload :one
SELECT id, company_id, uploaded_by, orders_data, products_data, couriers_data, locations_data,
       file_names, headers, preview, mapping, review_log, status, created_at, updated_at
FROM import_uploads
WHERE id = $1 AND company_id = $2;

-- name: SaveImportUploadMapping :one
UPDATE import_uploads
SET mapping = $3, status = 'mapped', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'uploaded'
RETURNING id, company_id, uploaded_by, file_names, headers, preview, mapping, review_log, status, created_at, updated_at;

-- name: StartMappedImportUpload :one
UPDATE import_uploads
SET status = 'importing', updated_at = NOW()
WHERE id = $1 AND company_id = $2 AND status = 'mapped'
RETURNING id, company_id, uploaded_by, orders_data, products_data, couriers_data, locations_data,
          file_names, headers, preview, mapping, review_log, status, created_at, updated_at;

-- name: CompleteImportUpload :one
UPDATE import_uploads
SET status = $3, review_log = $4, updated_at = NOW()
WHERE id = $1 AND company_id = $2
RETURNING id, company_id, uploaded_by, file_names, headers, preview, mapping, review_log, status, created_at, updated_at;