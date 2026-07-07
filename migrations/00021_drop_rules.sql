-- +goose Up
DROP TABLE IF EXISTS flow_rule_migrations;

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS chk_delivery_tasks_origin;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
        (origin_type = 'ai_digest' AND origin_id IS NOT NULL AND message_snapshot IS NOT NULL)
        OR (origin_type = 'flow' AND message_id IS NOT NULL AND origin_id IS NOT NULL AND origin_node_id IS NOT NULL)
    );

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS delivery_tasks_origin_type_check;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_origin_type_check
    CHECK (origin_type IN ('ai_digest','flow'));

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS delivery_tasks_rule_id_fkey;
DROP INDEX IF EXISTS idx_delivery_tasks_rule_unique;
DROP INDEX IF EXISTS idx_delivery_tasks_rule_id;

DROP TABLE IF EXISTS rule_filters;
DROP TABLE IF EXISTS rule_targets;
DROP TABLE IF EXISTS rule_sources;
DROP TABLE IF EXISTS rules;

-- +goose Down
CREATE TABLE rules (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL,
    enabled       boolean NOT NULL DEFAULT true,
    priority      integer NOT NULL DEFAULT 0,
    conditions    jsonb NOT NULL DEFAULT '[]'::jsonb,
    processors    jsonb NOT NULL DEFAULT '[]'::jsonb,
    stop_on_match boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rule_sources (
    rule_id   bigint NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    source_id bigint NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    PRIMARY KEY (rule_id, source_id)
);

CREATE TABLE rule_targets (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    rule_id     bigint NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    sink_id     bigint NOT NULL REFERENCES sinks(id) ON DELETE RESTRICT,
    template_id bigint REFERENCES templates(id) ON DELETE SET NULL,
    UNIQUE (rule_id, sink_id)
);

CREATE TABLE rule_filters (
    rule_id    bigint NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    filter_id  bigint NOT NULL REFERENCES filters(id) ON DELETE RESTRICT,
    sort_order integer NOT NULL DEFAULT 0,
    PRIMARY KEY (rule_id, filter_id)
);

CREATE TABLE flow_rule_migrations (
    rule_id    bigint PRIMARY KEY REFERENCES rules(id) ON DELETE CASCADE,
    flow_id    bigint NOT NULL UNIQUE REFERENCES flows(id) ON DELETE CASCADE,
    updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_rule_id_fkey FOREIGN KEY (rule_id) REFERENCES rules(id) ON DELETE CASCADE;

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS delivery_tasks_origin_type_check;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_origin_type_check
    CHECK (origin_type IN ('rule','ai_digest','flow'));

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS chk_delivery_tasks_origin;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
        (origin_type = 'rule' AND message_id IS NOT NULL AND rule_id IS NOT NULL)
        OR (origin_type = 'ai_digest' AND origin_id IS NOT NULL AND message_snapshot IS NOT NULL)
        OR (origin_type = 'flow' AND message_id IS NOT NULL AND origin_id IS NOT NULL AND origin_node_id IS NOT NULL)
    );
