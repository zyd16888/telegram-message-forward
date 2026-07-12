-- +goose Up
WITH ranked AS (
    SELECT id,
           row_number() OVER (PARTITION BY profile_id ORDER BY id DESC) AS position
    FROM ai_digest_runs
    WHERE profile_id IS NOT NULL
      AND trigger_type <> 'preview'
      AND status IN ('pending', 'running')
)
UPDATE ai_digest_runs
SET status = 'failed',
    finished_at = now(),
    error = '迁移时检测到同一 Profile 存在重复活动任务，已自动终止旧任务'
WHERE id IN (SELECT id FROM ranked WHERE position > 1);

CREATE UNIQUE INDEX uq_ai_digest_runs_active_profile
    ON ai_digest_runs (profile_id)
    WHERE profile_id IS NOT NULL
      AND trigger_type <> 'preview'
      AND status IN ('pending', 'running');

-- +goose Down
DROP INDEX IF EXISTS uq_ai_digest_runs_active_profile;
