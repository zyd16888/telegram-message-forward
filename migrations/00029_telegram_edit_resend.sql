-- +goose Up
ALTER TABLE messages
    ADD COLUMN content_revision integer NOT NULL DEFAULT 1,
    ADD COLUMN content_hash text NOT NULL DEFAULT '',
    ADD COLUMN edited_at timestamptz;

ALTER TABLE delivery_tasks
    ADD COLUMN message_revision integer NOT NULL DEFAULT 1,
    ADD COLUMN text_suffix text NOT NULL DEFAULT '';

DROP INDEX IF EXISTS idx_delivery_tasks_flow_unique;
CREATE UNIQUE INDEX idx_delivery_tasks_flow_unique
    ON delivery_tasks (message_id, origin_type, origin_id, origin_node_id, message_revision)
    WHERE origin_type = 'flow';

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_tasks_flow_unique;
CREATE UNIQUE INDEX idx_delivery_tasks_flow_unique
    ON delivery_tasks (message_id, origin_type, origin_id, origin_node_id)
    WHERE origin_type = 'flow';

ALTER TABLE delivery_tasks
    DROP COLUMN IF EXISTS text_suffix,
    DROP COLUMN IF EXISTS message_revision;

ALTER TABLE messages
    DROP COLUMN IF EXISTS edited_at,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS content_revision;
