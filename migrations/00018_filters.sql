-- +goose Up
-- 可复用「过滤器」：一组命名的匹配条件，供转发规则与 AI 整理共同引用。
CREATE TABLE filters (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    conditions  jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- 转发规则与 AI Profile 通过 filter_id 引用共享过滤器；为空表示使用各自内联条件。
ALTER TABLE rules
    ADD COLUMN filter_id bigint REFERENCES filters(id) ON DELETE SET NULL;
ALTER TABLE ai_digest_profiles
    ADD COLUMN filter_id bigint REFERENCES filters(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE ai_digest_profiles DROP COLUMN IF EXISTS filter_id;
ALTER TABLE rules DROP COLUMN IF EXISTS filter_id;
DROP TABLE IF EXISTS filters;
