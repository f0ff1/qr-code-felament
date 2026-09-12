BEGIN;

CREATE TABLE IF NOT EXISTS bambu_cloud_accounts (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL DEFAULT '',
    password TEXT NOT NULL DEFAULT '',
    token TEXT NOT NULL DEFAULT '',
    region TEXT NOT NULL DEFAULT 'us',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_bambu_cloud_accounts_email
    ON bambu_cloud_accounts (lower(email));

COMMIT;
