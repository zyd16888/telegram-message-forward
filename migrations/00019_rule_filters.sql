-- +goose Up
-- 转发规则支持引用多个共享过滤器；过滤器条件按 sort_order 合并后与规则其他条件一起按 AND 语义匹配。
CREATE TABLE rule_filters (
    rule_id    bigint NOT NULL REFERENCES rules(id) ON DELETE CASCADE,
    filter_id  bigint NOT NULL REFERENCES filters(id) ON DELETE RESTRICT,
    sort_order integer NOT NULL DEFAULT 0,
    PRIMARY KEY (rule_id, filter_id)
);

CREATE INDEX idx_rule_filters_filter_id ON rule_filters(filter_id);

ALTER TABLE rules DROP COLUMN IF EXISTS filter_id;

-- +goose Down
ALTER TABLE rules
    ADD COLUMN filter_id bigint REFERENCES filters(id) ON DELETE SET NULL;

DROP TABLE IF EXISTS rule_filters;
