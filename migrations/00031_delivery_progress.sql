-- +goose Up
ALTER TABLE delivery_tasks
    ADD COLUMN progress jsonb NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE sinks ADD COLUMN config_encrypted bytea;

-- +goose Down
ALTER TABLE sinks DROP COLUMN IF EXISTS config_encrypted;
ALTER TABLE delivery_tasks DROP COLUMN IF EXISTS progress;
