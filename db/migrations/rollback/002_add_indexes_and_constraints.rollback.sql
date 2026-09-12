BEGIN;

DROP INDEX IF EXISTS idx_products_material;
DROP INDEX IF EXISTS idx_printers_status;

ALTER TABLE spools
    ALTER COLUMN current_weight SET NOT NULL;

COMMIT;
