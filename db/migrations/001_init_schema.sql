BEGIN;

CREATE TABLE IF NOT EXISTS spools (
    id UUID PRIMARY KEY,
    qr_token TEXT NOT NULL UNIQUE,
    material TEXT NOT NULL,
    color TEXT NOT NULL,
    manufacturer TEXT NOT NULL,
    initial_weight INTEGER NOT NULL,
    current_weight INTEGER NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS printers (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    material TEXT NOT NULL,
    estimated_weight INTEGER NOT NULL,
    estimated_print_time TEXT NOT NULL,
    price DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS print_jobs (
    id UUID PRIMARY KEY,
    printer_id UUID NOT NULL,
    product_id UUID NOT NULL,
    spool_id UUID NOT NULL,
    status TEXT NOT NULL,
    progress DOUBLE PRECISION NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    estimated_weight INTEGER NOT NULL,
    consumed_weight INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS inventory_transactions (
    id UUID PRIMARY KEY,
    spool_id UUID NOT NULL,
    type TEXT NOT NULL,
    weight INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_spools_qr_token ON spools (qr_token);
CREATE INDEX IF NOT EXISTS idx_spools_status ON spools (status);
CREATE INDEX IF NOT EXISTS idx_print_jobs_status ON print_jobs (status);
CREATE INDEX IF NOT EXISTS idx_print_jobs_printer_id ON print_jobs (printer_id);
CREATE INDEX IF NOT EXISTS idx_inventory_transactions_spool_id ON inventory_transactions (spool_id);

COMMIT;
