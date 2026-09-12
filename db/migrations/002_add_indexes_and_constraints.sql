BEGIN;

ALTER TABLE print_jobs
    DROP CONSTRAINT IF EXISTS print_jobs_printer_id_fkey,
    DROP CONSTRAINT IF EXISTS print_jobs_product_id_fkey,
    DROP CONSTRAINT IF EXISTS print_jobs_spool_id_fkey;

ALTER TABLE inventory_transactions
    DROP CONSTRAINT IF EXISTS inventory_transactions_spool_id_fkey;

ALTER TABLE spools
    ALTER COLUMN current_weight DROP NOT NULL;

CREATE INDEX IF NOT EXISTS idx_products_material ON products (material);
CREATE INDEX IF NOT EXISTS idx_printers_status ON printers (status);

COMMIT;
