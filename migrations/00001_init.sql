-- +goose Up
CREATE TABLE accounts (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name                 text NOT NULL,
    phone_number         text NOT NULL,
    app_id               integer NOT NULL,
    app_hash_encrypted   bytea,
    session_encrypted    bytea,
    proxy_config         jsonb NOT NULL DEFAULT '{}'::jsonb,
    status               text NOT NULL DEFAULT 'inactive'
                         CHECK (status IN ('inactive','logging_in','active','error','banned')),
    last_login_at        timestamptz,
    last_error           text,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE telegram_peers (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id   bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    peer_type    text NOT NULL CHECK (peer_type IN ('user','chat','channel')),
    peer_id      bigint NOT NULL,
    access_hash  bigint NOT NULL,
    username     text,
    title        text,
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, peer_type, peer_id)
);

CREATE TABLE sources (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id       bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    peer_type        text NOT NULL CHECK (peer_type IN ('user','chat','channel')),
    peer_id          bigint NOT NULL,
    name             text NOT NULL,
    username         text,
    enabled          boolean NOT NULL DEFAULT true,
    config           jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_message_id  bigint NOT NULL DEFAULT 0,
    last_synced_at   timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, peer_type, peer_id)
);

CREATE TABLE sinks (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type              text NOT NULL,
    name              text NOT NULL,
    enabled           boolean NOT NULL DEFAULT true,
    config            jsonb NOT NULL DEFAULT '{}'::jsonb,
    secret_encrypted  bytea,
    capabilities      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE templates (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL,
    format     text NOT NULL CHECK (format IN ('text','markdown','html')),
    content    text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

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
    sink_id     bigint NOT NULL REFERENCES sinks(id) ON DELETE CASCADE,
    template_id bigint REFERENCES templates(id) ON DELETE SET NULL,
    UNIQUE (rule_id, sink_id)
);

CREATE TABLE messages (
    id                   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    source_id            bigint NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    external_message_id  bigint NOT NULL,
    grouped_id           bigint,
    message_type         text NOT NULL,
    sender_peer_type     text,
    sender_id            bigint,
    sender_name          text,
    text                 text,
    media                jsonb NOT NULL DEFAULT '[]'::jsonb,
    links                jsonb NOT NULL DEFAULT '[]'::jsonb,
    original_url         text,
    raw_payload          jsonb,
    sent_at              timestamptz,
    received_at          timestamptz NOT NULL DEFAULT now(),
    created_at           timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_id, external_message_id)
);
CREATE INDEX idx_messages_source_sent_at ON messages (source_id, sent_at);

CREATE TABLE delivery_tasks (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id     bigint NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    rule_id        bigint NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    sink_id        bigint NOT NULL REFERENCES sinks(id) ON DELETE CASCADE,
    template_id    bigint REFERENCES templates(id) ON DELETE SET NULL,
    status         text NOT NULL DEFAULT 'pending'
                   CHECK (status IN ('pending','processing','success','failed','retrying','dead','cancelled')),
    attempt_count  integer NOT NULL DEFAULT 0,
    max_attempts   integer NOT NULL DEFAULT 3,
    next_retry_at  timestamptz,
    locked_at      timestamptz,
    locked_by      text,
    last_error     text,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (message_id, rule_id, sink_id)
);
CREATE INDEX idx_delivery_tasks_status_next_retry ON delivery_tasks (status, next_retry_at);
CREATE INDEX idx_delivery_tasks_status_locked ON delivery_tasks (status, locked_at);

CREATE TABLE delivery_attempts (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    delivery_task_id  bigint NOT NULL REFERENCES delivery_tasks(id) ON DELETE CASCADE,
    attempt_no        integer NOT NULL,
    status            text NOT NULL CHECK (status IN ('success','failed')),
    request_summary   jsonb,
    response_summary  jsonb,
    error             text,
    started_at        timestamptz,
    finished_at       timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_delivery_attempts_task ON delivery_attempts (delivery_task_id);

CREATE TABLE api_tokens (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name         text NOT NULL,
    token_hash   text NOT NULL UNIQUE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    revoked_at   timestamptz
);

CREATE TABLE settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS delivery_attempts;
DROP TABLE IF EXISTS delivery_tasks;
DROP TABLE IF EXISTS messages;
DROP TABLE IF EXISTS rule_targets;
DROP TABLE IF EXISTS rule_sources;
DROP TABLE IF EXISTS rules;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS sinks;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS telegram_peers;
DROP TABLE IF EXISTS accounts;
