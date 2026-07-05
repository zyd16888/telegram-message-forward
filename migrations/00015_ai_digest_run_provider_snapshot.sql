-- +goose Up
ALTER TABLE ai_digest_runs
    ADD COLUMN IF NOT EXISTS provider_id TEXT,
    ADD COLUMN IF NOT EXISTS provider_name TEXT;

-- +goose Down
ALTER TABLE ai_digest_runs
    DROP COLUMN IF EXISTS provider_name,
    DROP COLUMN IF EXISTS provider_id;
