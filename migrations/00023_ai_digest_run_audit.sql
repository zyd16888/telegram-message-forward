-- +goose Up
ALTER TABLE ai_digest_runs
    ADD COLUMN system_prompt TEXT NOT NULL DEFAULT '',
    ADD COLUMN user_prompt TEXT NOT NULL DEFAULT '',
    ADD COLUMN request_config JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE ai_digest_run_items
    ADD COLUMN message_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;

UPDATE ai_digest_run_items AS item
SET message_snapshot = jsonb_build_object(
    'id', msg.id,
    'source_id', msg.source_id,
    'external_message_id', msg.external_message_id,
    'grouped_id', msg.grouped_id,
    'message_type', msg.message_type,
    'sender_peer_type', msg.sender_peer_type,
    'sender_id', msg.sender_id,
    'sender_name', msg.sender_name,
    'text', msg.text,
    'media', (
        SELECT COALESCE(
            jsonb_agg(media_item - 'local_path' - 'storage_key' - 'download_status' - 'download_error'),
            '[]'::jsonb
        )
        FROM jsonb_array_elements(
            CASE WHEN jsonb_typeof(msg.media) = 'array' THEN msg.media ELSE '[]'::jsonb END
        ) AS media_item
    ),
    'links', COALESCE(msg.links, '[]'::jsonb),
    'original_url', msg.original_url,
    'sent_at', msg.sent_at,
    'received_at', msg.received_at,
    'created_at', msg.created_at
)
FROM messages AS msg
WHERE msg.id = item.message_id;

ALTER TABLE ai_digest_run_items
    DROP CONSTRAINT IF EXISTS ai_digest_run_items_message_id_fkey,
    DROP CONSTRAINT IF EXISTS ai_digest_run_items_source_id_fkey;

-- +goose Down
DELETE FROM ai_digest_run_items AS item
WHERE NOT EXISTS (SELECT 1 FROM messages WHERE messages.id = item.message_id)
   OR NOT EXISTS (SELECT 1 FROM sources WHERE sources.id = item.source_id);

ALTER TABLE ai_digest_run_items
    ADD CONSTRAINT ai_digest_run_items_message_id_fkey
        FOREIGN KEY (message_id) REFERENCES messages(id) ON DELETE CASCADE,
    ADD CONSTRAINT ai_digest_run_items_source_id_fkey
        FOREIGN KEY (source_id) REFERENCES sources(id) ON DELETE CASCADE;

ALTER TABLE ai_digest_run_items
    DROP COLUMN message_snapshot;

ALTER TABLE ai_digest_runs
    DROP COLUMN request_config,
    DROP COLUMN user_prompt,
    DROP COLUMN system_prompt;
