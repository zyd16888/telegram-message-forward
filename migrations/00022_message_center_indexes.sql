-- +goose Up
CREATE INDEX IF NOT EXISTS idx_messages_received_id
    ON messages (received_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_delivery_tasks_message_status
    ON delivery_tasks (message_id, status)
    WHERE message_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_tasks_message_status;
DROP INDEX IF EXISTS idx_messages_received_id;
