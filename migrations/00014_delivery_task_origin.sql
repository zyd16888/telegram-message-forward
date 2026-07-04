-- +goose Up
ALTER TABLE delivery_tasks ALTER COLUMN message_id DROP NOT NULL;
ALTER TABLE delivery_tasks ALTER COLUMN rule_id DROP NOT NULL;
ALTER TABLE delivery_tasks
    ADD COLUMN origin_type text NOT NULL DEFAULT 'rule'
    CHECK (origin_type IN ('rule','ai_digest'));
ALTER TABLE delivery_tasks ADD COLUMN origin_id bigint;

ALTER TABLE delivery_tasks
    DROP CONSTRAINT IF EXISTS delivery_tasks_message_id_rule_id_sink_id_key;
CREATE UNIQUE INDEX idx_delivery_tasks_rule_unique
    ON delivery_tasks (message_id, rule_id, sink_id)
    WHERE origin_type = 'rule';
CREATE INDEX idx_delivery_tasks_origin
    ON delivery_tasks (origin_type, origin_id);

ALTER TABLE delivery_tasks ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
    (origin_type = 'rule' AND message_id IS NOT NULL AND rule_id IS NOT NULL)
    OR (origin_type = 'ai_digest' AND origin_id IS NOT NULL AND message_snapshot IS NOT NULL)
);

-- +goose Down
ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS chk_delivery_tasks_origin;
DROP INDEX IF EXISTS idx_delivery_tasks_origin;
DROP INDEX IF EXISTS idx_delivery_tasks_rule_unique;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_message_id_rule_id_sink_id_key UNIQUE (message_id, rule_id, sink_id);
ALTER TABLE delivery_tasks DROP COLUMN IF EXISTS origin_id;
ALTER TABLE delivery_tasks DROP COLUMN IF EXISTS origin_type;
ALTER TABLE delivery_tasks ALTER COLUMN rule_id SET NOT NULL;
ALTER TABLE delivery_tasks ALTER COLUMN message_id SET NOT NULL;
