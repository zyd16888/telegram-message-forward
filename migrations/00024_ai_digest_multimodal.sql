-- +goose Up
ALTER TABLE ai_digest_profiles
    ADD COLUMN multimodal JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE ai_digest_runs
    ADD COLUMN media_audit JSONB NOT NULL DEFAULT '[]'::jsonb;

-- +goose Down
ALTER TABLE ai_digest_runs DROP COLUMN media_audit;
ALTER TABLE ai_digest_profiles DROP COLUMN multimodal;
