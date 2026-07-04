-- +goose Up
CREATE TABLE ai_digest_profiles (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name               text NOT NULL,
    enabled            boolean NOT NULL DEFAULT false,
    source_ids          jsonb NOT NULL DEFAULT '[]'::jsonb,
    conditions          jsonb NOT NULL DEFAULT '[]'::jsonb,
    schedule            jsonb NOT NULL DEFAULT '{}'::jsonb,
    "window"            jsonb NOT NULL DEFAULT '{}'::jsonb,
    dedupe              jsonb NOT NULL DEFAULT '{}'::jsonb,
    prompt_template     text NOT NULL,
    output_format       text NOT NULL DEFAULT 'markdown'
                       CHECK (output_format IN ('text','markdown','html')),
    target_sink_ids     jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_config        jsonb NOT NULL DEFAULT '{}'::jsonb,
    limits              jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE ai_digest_runs (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    profile_id            bigint REFERENCES ai_digest_profiles(id) ON DELETE CASCADE,
    status                text NOT NULL
                          CHECK (status IN ('pending','running','success','failed','cancelled')),
    trigger_type          text NOT NULL DEFAULT 'manual'
                          CHECK (trigger_type IN ('manual','schedule','preview')),
    window_start          timestamptz NOT NULL,
    window_end            timestamptz NOT NULL,
    input_message_count   integer NOT NULL DEFAULT 0,
    included_count        integer NOT NULL DEFAULT 0,
    excluded_count        integer NOT NULL DEFAULT 0,
    delivery_task_ids     jsonb NOT NULL DEFAULT '[]'::jsonb,
    model_name            text,
    token_usage           jsonb NOT NULL DEFAULT '{}'::jsonb,
    error                 text,
    started_at            timestamptz,
    finished_at           timestamptz,
    created_at            timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_ai_digest_runs_profile ON ai_digest_runs (profile_id, created_at DESC);

CREATE TABLE ai_digest_run_items (
    run_id          bigint NOT NULL REFERENCES ai_digest_runs(id) ON DELETE CASCADE,
    message_id      bigint NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    source_id       bigint NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    included        boolean NOT NULL DEFAULT true,
    reason          text,
    score           double precision,
    sort_order      integer NOT NULL DEFAULT 0,
    PRIMARY KEY (run_id, message_id)
);
CREATE INDEX idx_ai_digest_run_items_message ON ai_digest_run_items (message_id);

CREATE TABLE ai_digest_outputs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id          bigint NOT NULL REFERENCES ai_digest_runs(id) ON DELETE CASCADE,
    format          text NOT NULL DEFAULT 'markdown'
                    CHECK (format IN ('text','markdown','html')),
    title           text,
    content         text NOT NULL,
    raw_response    jsonb,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_ai_digest_outputs_run ON ai_digest_outputs (run_id);

CREATE INDEX idx_messages_source_received_at ON messages (source_id, received_at);

-- +goose Down
DROP INDEX IF EXISTS idx_messages_source_received_at;
DROP TABLE IF EXISTS ai_digest_outputs;
DROP TABLE IF EXISTS ai_digest_run_items;
DROP TABLE IF EXISTS ai_digest_runs;
DROP TABLE IF EXISTS ai_digest_profiles;
