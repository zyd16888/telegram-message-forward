-- +goose Up
ALTER TABLE ai_digest_runs
    ADD COLUMN prompt_message_count integer NOT NULL DEFAULT 0,
    ADD COLUMN prompt_omitted_count integer NOT NULL DEFAULT 0,
    ADD COLUMN prompt_chars integer NOT NULL DEFAULT 0,
    ADD COLUMN profile_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE ai_digest_runs
    DROP COLUMN IF EXISTS profile_snapshot,
    DROP COLUMN IF EXISTS prompt_chars,
    DROP COLUMN IF EXISTS prompt_omitted_count,
    DROP COLUMN IF EXISTS prompt_message_count;
