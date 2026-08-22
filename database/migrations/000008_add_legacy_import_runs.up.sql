ALTER TABLE orders ADD COLUMN legacy_source_key TEXT;

CREATE UNIQUE INDEX orders_company_legacy_source_key_idx
    ON orders(company_id, legacy_source_key)
    WHERE legacy_source_key IS NOT NULL;

CREATE TABLE legacy_import_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL REFERENCES companies(id),
    triggered_by UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'completed', 'failed')),
    rows_read INTEGER NOT NULL DEFAULT 0,
    rows_inserted INTEGER NOT NULL DEFAULT 0,
    rows_skipped INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    UNIQUE(company_id),
    FOREIGN KEY (triggered_by, company_id) REFERENCES users(id, company_id)
);

CREATE INDEX legacy_import_runs_company_id_idx ON legacy_import_runs(company_id);