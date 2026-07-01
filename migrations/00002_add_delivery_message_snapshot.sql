-- +goose Up
ALTER TABLE delivery_tasks
    ADD COLUMN message_snapshot jsonb;

-- +goose Down
ALTER TABLE delivery_tasks
    DROP COLUMN IF EXISTS message_snapshot;
