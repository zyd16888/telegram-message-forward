-- +goose Up
-- Telegram 登录 flow，持久化验证码/扫码登录过程，支持服务重启后恢复、可过期清理。
-- 敏感字段（phone_code_hash、qr_token）加密存储，验证码明文与 2FA 密码永不落库。
CREATE TABLE telegram_login_flows (
    id                        bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    flow_id                   text NOT NULL UNIQUE,
    account_id                bigint NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    method                    text NOT NULL CHECK (method IN ('phone_code','qr')),
    status                    text NOT NULL,
    current_step              text NOT NULL DEFAULT '',
    phone_code_hash_encrypted bytea,
    qr_token_encrypted        bytea,
    dc_id                     integer NOT NULL DEFAULT 0,
    expires_at                timestamptz NOT NULL,
    last_error                text,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now(),
    completed_at              timestamptz
);
CREATE INDEX idx_telegram_login_flows_account ON telegram_login_flows (account_id);
CREATE INDEX idx_telegram_login_flows_status ON telegram_login_flows (status, expires_at);

-- +goose Down
DROP TABLE IF EXISTS telegram_login_flows;
