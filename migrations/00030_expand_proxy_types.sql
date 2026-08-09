-- +goose Up
ALTER TABLE proxy_configs DROP CONSTRAINT proxy_configs_type_check;
ALTER TABLE proxy_configs
    ADD CONSTRAINT proxy_configs_type_check
    CHECK (type IN ('socks5', 'http', 'https'));

-- +goose Down
ALTER TABLE proxy_configs DROP CONSTRAINT proxy_configs_type_check;
ALTER TABLE proxy_configs
    ADD CONSTRAINT proxy_configs_type_check
    CHECK (type IN ('socks5'));
