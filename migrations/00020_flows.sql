-- +goose Up
CREATE TABLE flows (
    id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name          text NOT NULL,
    enabled       boolean NOT NULL DEFAULT true,
    priority      integer NOT NULL DEFAULT 0,
    stop_on_match boolean NOT NULL DEFAULT false,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE flow_nodes (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    flow_id     bigint NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    type        text NOT NULL CHECK (type IN ('source','filter','processor','target')),
    ref_id      bigint,
    config      jsonb NOT NULL DEFAULT '{}'::jsonb,
    template_id bigint REFERENCES templates(id) ON DELETE SET NULL,
    pos_x       double precision NOT NULL DEFAULT 0,
    pos_y       double precision NOT NULL DEFAULT 0
);

CREATE TABLE flow_edges (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    flow_id      bigint NOT NULL REFERENCES flows(id) ON DELETE CASCADE,
    from_node_id bigint NOT NULL REFERENCES flow_nodes(id) ON DELETE CASCADE,
    to_node_id   bigint NOT NULL REFERENCES flow_nodes(id) ON DELETE CASCADE,
    UNIQUE (flow_id, from_node_id, to_node_id)
);

CREATE TABLE flow_rule_migrations (
    rule_id    bigint PRIMARY KEY REFERENCES rules(id) ON DELETE CASCADE,
    flow_id    bigint NOT NULL UNIQUE REFERENCES flows(id) ON DELETE CASCADE,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_flow_nodes_flow ON flow_nodes (flow_id);
CREATE INDEX idx_flow_nodes_type_ref ON flow_nodes (type, ref_id);
CREATE INDEX idx_flow_edges_flow_from ON flow_edges (flow_id, from_node_id);
CREATE INDEX idx_flow_edges_flow_to ON flow_edges (flow_id, to_node_id);

ALTER TABLE delivery_tasks
    ADD COLUMN origin_node_id bigint;

ALTER TABLE delivery_tasks
    DROP CONSTRAINT IF EXISTS delivery_tasks_origin_type_check;

ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_origin_type_check
    CHECK (origin_type IN ('rule','ai_digest','flow'));

ALTER TABLE delivery_tasks
    DROP CONSTRAINT IF EXISTS chk_delivery_tasks_origin;

ALTER TABLE delivery_tasks
    ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
        (origin_type = 'rule' AND message_id IS NOT NULL AND rule_id IS NOT NULL)
        OR (origin_type = 'ai_digest' AND origin_id IS NOT NULL AND message_snapshot IS NOT NULL)
        OR (origin_type = 'flow' AND message_id IS NOT NULL AND origin_id IS NOT NULL AND origin_node_id IS NOT NULL)
    );

CREATE UNIQUE INDEX idx_delivery_tasks_flow_unique
    ON delivery_tasks (message_id, origin_type, origin_id, origin_node_id)
    WHERE origin_type = 'flow';

-- +goose Down
DROP INDEX IF EXISTS idx_delivery_tasks_flow_unique;

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS chk_delivery_tasks_origin;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT chk_delivery_tasks_origin CHECK (
        (origin_type = 'rule' AND message_id IS NOT NULL AND rule_id IS NOT NULL)
        OR (origin_type = 'ai_digest' AND origin_id IS NOT NULL AND message_snapshot IS NOT NULL)
    );

ALTER TABLE delivery_tasks DROP CONSTRAINT IF EXISTS delivery_tasks_origin_type_check;
ALTER TABLE delivery_tasks
    ADD CONSTRAINT delivery_tasks_origin_type_check
    CHECK (origin_type IN ('rule','ai_digest'));

ALTER TABLE delivery_tasks DROP COLUMN IF EXISTS origin_node_id;

DROP TABLE IF EXISTS flow_edges;
DROP TABLE IF EXISTS flow_nodes;
DROP TABLE IF EXISTS flow_rule_migrations;
DROP TABLE IF EXISTS flows;
