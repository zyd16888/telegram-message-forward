-- +goose Up
ALTER TABLE sinks ADD COLUMN last_test_at timestamptz;
ALTER TABLE sinks ADD COLUMN last_test_success boolean NOT NULL DEFAULT false;
ALTER TABLE sinks ADD COLUMN last_test_error text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE sinks DROP COLUMN last_test_error;
ALTER TABLE sinks DROP COLUMN last_test_success;
ALTER TABLE sinks DROP COLUMN last_test_at;
