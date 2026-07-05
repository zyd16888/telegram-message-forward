-- +goose Up
ALTER TABLE ai_digest_profiles
    ADD COLUMN output_template text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE ai_digest_profiles
    DROP COLUMN IF EXISTS output_template;
