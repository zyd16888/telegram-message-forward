-- +goose Up
CREATE TABLE admin_users (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    username       text NOT NULL UNIQUE,
    password_hash  text NOT NULL,
    active         boolean NOT NULL DEFAULT true,
    last_login_at  timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_sessions (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       bigint NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    token_hash    text NOT NULL UNIQUE,
    expires_at    timestamptz NOT NULL,
    last_used_at  timestamptz,
    revoked_at    timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_admin_sessions_token_active ON admin_sessions (token_hash, expires_at) WHERE revoked_at IS NULL;

CREATE TABLE telegram_apps (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                text NOT NULL,
    app_id              integer NOT NULL,
    app_hash_encrypted  bytea NOT NULL,
    enabled             boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE proxy_configs (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                text NOT NULL,
    type                text NOT NULL CHECK (type IN ('socks5')),
    addr                text NOT NULL,
    username            text,
    password_encrypted  bytea,
    enabled             boolean NOT NULL DEFAULT true,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE accounts ADD COLUMN telegram_app_id bigint REFERENCES telegram_apps(id) ON DELETE RESTRICT;
ALTER TABLE accounts ADD COLUMN proxy_id bigint REFERENCES proxy_configs(id) ON DELETE SET NULL;

INSERT INTO telegram_apps (name, app_id, app_hash_encrypted, enabled)
SELECT DISTINCT 'App ' || app_id::text, app_id, app_hash_encrypted, true
FROM accounts
WHERE app_id <> 0 AND app_hash_encrypted IS NOT NULL;

UPDATE accounts a
SET telegram_app_id = ta.id
FROM telegram_apps ta
WHERE a.telegram_app_id IS NULL
  AND a.app_id = ta.app_id
  AND a.app_hash_encrypted = ta.app_hash_encrypted;

-- +goose Down
ALTER TABLE accounts DROP COLUMN IF EXISTS proxy_id;
ALTER TABLE accounts DROP COLUMN IF EXISTS telegram_app_id;
DROP TABLE IF EXISTS proxy_configs;
DROP TABLE IF EXISTS telegram_apps;
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
