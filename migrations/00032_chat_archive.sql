-- +goose Up

-- pg_trgm 用于中文子串检索：Postgres 的 to_tsvector 不做中文分词，
-- 全文索引对中文基本无效，因此归档检索走 ILIKE + trigram GIN 索引。
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- chat_archives 是一个被归档的会话。
-- 与 sources 解耦：归档不要求该会话被配置为监听源。
CREATE TABLE chat_archives (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    account_id     bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    peer_type      text NOT NULL CHECK (peer_type IN ('user','chat','channel')),
    peer_id        bigint NOT NULL,
    peer_name      text NOT NULL DEFAULT '',
    peer_username  text NOT NULL DEFAULT '',
    -- 已覆盖的消息 id 区间，用于增量补拉。
    min_message_id bigint NOT NULL DEFAULT 0,
    max_message_id bigint NOT NULL DEFAULT 0,
    message_count  bigint NOT NULL DEFAULT 0,
    media_count    bigint NOT NULL DEFAULT 0,
    last_synced_at timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (account_id, peer_type, peer_id)
);

-- chat_archive_messages 比 messages 表宽：保留分析所需的回复链、方向、
-- 转发来源、服务消息等字段。归档数据不进 Flow、不产生 delivery_tasks，
-- 也不受 messages_retention_days 保留期清理影响。
CREATE TABLE chat_archive_messages (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    archive_id          bigint NOT NULL REFERENCES chat_archives(id) ON DELETE CASCADE,
    message_id          bigint NOT NULL,
    grouped_id          bigint,
    reply_to_message_id bigint,
    -- outgoing=true 表示本账号发出的消息。
    -- 不用 out 作列名：out 是 Postgres 关键字，裸用在 SQL 片段里容易踩坑。
    outgoing            boolean NOT NULL DEFAULT false,
    sender_peer_type    text NOT NULL DEFAULT '',
    sender_id           bigint NOT NULL DEFAULT 0,
    sender_name         text NOT NULL DEFAULT '',
    sender_username     text NOT NULL DEFAULT '',
    message_type        text NOT NULL DEFAULT 'text',
    text                text NOT NULL DEFAULT '',
    -- entities 保留富文本 offset（链接/引用/mention），供分析还原原文结构。
    entities            jsonb NOT NULL DEFAULT '[]'::jsonb,
    media               jsonb NOT NULL DEFAULT '[]'::jsonb,
    fwd_from            jsonb,
    reactions           jsonb,
    -- service_action 非空表示这是服务消息（入群、改名等），text 为空。
    service_action      text NOT NULL DEFAULT '',
    views               integer NOT NULL DEFAULT 0,
    date                timestamptz,
    edit_date           timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    UNIQUE (archive_id, message_id)
);

CREATE INDEX idx_chat_archive_messages_archive_date
    ON chat_archive_messages (archive_id, date);
CREATE INDEX idx_chat_archive_messages_archive_sender
    ON chat_archive_messages (archive_id, sender_id);
CREATE INDEX idx_chat_archive_messages_text_trgm
    ON chat_archive_messages USING GIN (text gin_trgm_ops);

-- chat_export_jobs 是一次拉取/渲染任务。
CREATE TABLE chat_export_jobs (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    archive_id       bigint NOT NULL REFERENCES chat_archives(id) ON DELETE CASCADE,
    status           text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','running','succeeded','failed','cancelled')),
    from_date        timestamptz,
    to_date          timestamptz,
    include_media    boolean NOT NULL DEFAULT false,
    media_max_bytes  bigint NOT NULL DEFAULT 0,
    max_messages     integer NOT NULL DEFAULT 0,
    fetched_count    integer NOT NULL DEFAULT 0,
    media_count      integer NOT NULL DEFAULT 0,
    -- cursor_offset_id 是断点续传游标：中断后从该处继续翻页。
    cursor_offset_id bigint NOT NULL DEFAULT 0,
    -- cancel_requested 由 API 置位，执行器在页边界检查后停止。
    cancel_requested boolean NOT NULL DEFAULT false,
    last_error       text NOT NULL DEFAULT '',
    started_at       timestamptz,
    finished_at      timestamptz,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_chat_export_jobs_archive_created
    ON chat_export_jobs (archive_id, created_at DESC);
-- 同一归档同时只允许一个在跑的任务，避免并发拉取互相顶游标。
CREATE UNIQUE INDEX idx_chat_export_jobs_active
    ON chat_export_jobs (archive_id)
    WHERE status IN ('pending','running');

-- +goose Down
DROP INDEX IF EXISTS idx_chat_export_jobs_active;
DROP INDEX IF EXISTS idx_chat_export_jobs_archive_created;
DROP TABLE IF EXISTS chat_export_jobs;

DROP INDEX IF EXISTS idx_chat_archive_messages_text_trgm;
DROP INDEX IF EXISTS idx_chat_archive_messages_archive_sender;
DROP INDEX IF EXISTS idx_chat_archive_messages_archive_date;
DROP TABLE IF EXISTS chat_archive_messages;

DROP TABLE IF EXISTS chat_archives;
