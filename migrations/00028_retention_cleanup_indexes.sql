-- +goose Up
CREATE INDEX IF NOT EXISTS idx_delivery_tasks_terminal_created_id
    ON delivery_tasks (created_at, id)
    WHERE status IN ('success', 'failed', 'dead', 'cancelled');

CREATE INDEX IF NOT EXISTS idx_ai_digest_runs_created_id
    ON ai_digest_runs (created_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_ai_digest_runs_created_id;
DROP INDEX IF EXISTS idx_delivery_tasks_terminal_created_id;
