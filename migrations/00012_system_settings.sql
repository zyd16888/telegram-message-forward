-- +goose Up
-- settings 表在 00001 已预留（key/value/updated_at），此处补充敏感字段加密列。
ALTER TABLE settings ADD COLUMN secret_encrypted bytea;

-- +goose Down
ALTER TABLE settings DROP COLUMN secret_encrypted;
