CREATE TABLE import_uploads (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id UUID NOT NULL,
    uploaded_by UUID NOT NULL,
    orders_data BYTEA NOT NULL,
    products_data BYTEA,
    couriers_data BYTEA,
    locations_data BYTEA,
    file_names JSONB NOT NULL,
    headers JSONB NOT NULL,
    preview JSONB NOT NULL,
    mapping JSONB,
    review_log JSONB,
    status TEXT NOT NULL DEFAULT 'uploaded' CHECK (status IN ('uploaded', 'mapped', 'importing', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    FOREIGN KEY (uploaded_by, company_id) REFERENCES users(id, company_id),
    FOREIGN KEY (company_id) REFERENCES companies(id)
);

CREATE INDEX import_uploads_company_id_idx ON import_uploads(company_id);