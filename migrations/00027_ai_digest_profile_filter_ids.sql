-- +goose Up
-- AI Profile 支持多过滤器引用（AND 语义），兼容旧 filter_id。
ALTER TABLE ai_digest_profiles
    ADD COLUMN IF NOT EXISTS filter_ids jsonb NOT NULL DEFAULT '[]'::jsonb;

UPDATE ai_digest_profiles
SET filter_ids = jsonb_build_array(filter_id)
WHERE filter_id IS NOT NULL
  AND (filter_ids IS NULL OR filter_ids = '[]'::jsonb);

-- +goose Down
ALTER TABLE ai_digest_profiles DROP COLUMN IF EXISTS filter_ids;
