BEGIN;

-- Company / locations / users
CREATE TABLE IF NOT EXISTS organizations (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS sites (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    address TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    username TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT users_role_check CHECK (role IN ('admin', 'operator', 'viewer'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_lower ON users (lower(username));

CREATE TABLE IF NOT EXISTS user_sites (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    site_id UUID NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, site_id)
);

CREATE TABLE IF NOT EXISTS password_reset_requests (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    CONSTRAINT password_reset_status_check CHECK (status IN ('open', 'done'))
);

CREATE INDEX IF NOT EXISTS idx_password_reset_open ON password_reset_requests (status, created_at)
    WHERE status = 'open';

-- Scope operational data to a site
ALTER TABLE spools ADD COLUMN IF NOT EXISTS site_id UUID;
ALTER TABLE printers ADD COLUMN IF NOT EXISTS site_id UUID;
ALTER TABLE print_jobs ADD COLUMN IF NOT EXISTS site_id UUID;
ALTER TABLE inventory_transactions ADD COLUMN IF NOT EXISTS site_id UUID;
ALTER TABLE bambu_cloud_accounts ADD COLUMN IF NOT EXISTS site_id UUID;
ALTER TABLE products ADD COLUMN IF NOT EXISTS organization_id UUID;

-- Bootstrap default org + site (fixed UUIDs match internal/domain/org)
INSERT INTO organizations (id, name, created_at, updated_at)
VALUES ('00000000-0000-4000-8000-000000000001', 'Default Company', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

INSERT INTO sites (id, organization_id, name, address, created_at, updated_at)
VALUES (
    '00000000-0000-4000-8000-000000000002',
    '00000000-0000-4000-8000-000000000001',
    'Основной склад',
    '',
    NOW(),
    NOW()
)
ON CONFLICT (id) DO NOTHING;

UPDATE spools SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;
UPDATE printers SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;
UPDATE print_jobs SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;
UPDATE inventory_transactions SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;
UPDATE bambu_cloud_accounts SET site_id = '00000000-0000-4000-8000-000000000002' WHERE site_id IS NULL;
UPDATE products SET organization_id = '00000000-0000-4000-8000-000000000001' WHERE organization_id IS NULL;

ALTER TABLE spools ALTER COLUMN site_id SET NOT NULL;
ALTER TABLE printers ALTER COLUMN site_id SET NOT NULL;
ALTER TABLE print_jobs ALTER COLUMN site_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_spools_site_id ON spools (site_id);
CREATE INDEX IF NOT EXISTS idx_printers_site_id ON printers (site_id);
CREATE INDEX IF NOT EXISTS idx_print_jobs_site_id ON print_jobs (site_id);

COMMIT;
