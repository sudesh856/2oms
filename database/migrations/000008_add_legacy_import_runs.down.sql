DROP TABLE IF EXISTS legacy_import_runs;
DROP INDEX IF EXISTS orders_company_legacy_source_key_idx;
ALTER TABLE orders DROP COLUMN IF EXISTS legacy_source_key;