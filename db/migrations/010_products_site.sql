ALTER TABLE products ADD COLUMN IF NOT EXISTS site_id UUID;

UPDATE products SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;

ALTER TABLE products ALTER COLUMN site_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_products_site_id ON products (site_id);
