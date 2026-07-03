-- +goose Up
ALTER TABLE sources ADD COLUMN source_type text NOT NULL DEFAULT 'telegram';
ALTER TABLE sources ALTER COLUMN account_id DROP NOT NULL;
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_peer_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_peer_type_check CHECK (
    (source_type = 'telegram' AND peer_type IN ('user', 'chat', 'channel'))
    OR (source_type = 'rss' AND peer_type = 'feed')
    OR (source_type = 'webhook' AND peer_type = 'webhook')
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sources_external_unique
    ON sources (source_type, peer_type, peer_id)
    WHERE source_type <> 'telegram';

-- +goose Down
DROP INDEX IF EXISTS idx_sources_external_unique;
ALTER TABLE sources DROP CONSTRAINT IF EXISTS sources_peer_type_check;
ALTER TABLE sources ADD CONSTRAINT sources_peer_type_check CHECK (peer_type IN ('user', 'chat', 'channel'));
ALTER TABLE sources ALTER COLUMN account_id SET NOT NULL;
ALTER TABLE sources DROP COLUMN source_type;
