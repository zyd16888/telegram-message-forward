-- +goose Up
-- 输出结构模板抽成可复用实体：多个 profile 可引用同一模板。
CREATE TABLE ai_digest_output_templates (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name        text NOT NULL,
    description text NOT NULL DEFAULT '',
    format      text NOT NULL DEFAULT 'markdown'
                CHECK (format IN ('text','markdown','html')),
    content     text NOT NULL,
    built_in    boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

-- profile 通过 output_template_id 引用共享模板；为空表示使用内联自定义 output_template。
ALTER TABLE ai_digest_profiles
    ADD COLUMN output_template_id bigint
        REFERENCES ai_digest_output_templates(id) ON DELETE SET NULL;

-- 内置模板种子（与旧版前端预设一致），built_in=true 供 UI 标记且不允许删除。
-- +goose StatementBegin
INSERT INTO ai_digest_output_templates (name, description, format, content, built_in) VALUES
('通用简报', '适合大多数消息窗口，包含总结、摘要、分类和待关注事项。', 'markdown', $tmpl$# {{profile_name}}

## 一句话总结
用 1 段话概括本窗口最重要的信息。

## 重点摘要
- 列出 3-7 条重点，每条都标注来源编号，例如 [#1]。
- 合并重复消息，不要重复罗列同一件事。

## 分类整理
按主题分组整理，每组包含关键事实、背景线索和来源编号。

## 待关注事项
列出需要继续关注的问题、风险、待办或后续进展。

## 说明
以上内容仅基于本窗口内消息整理，未使用外部事实补全。$tmpl$, true),
('新闻简报', '适合资讯源、新闻频道和 RSS 摘要。', 'markdown', $tmpl$# 今日新闻总结

## 今日要点
用 3-7 条概括最重要的新闻，每条都标注来源编号。

## 分主题整理
按主题分组，例如国际、国内、科技、财经、行业动态；没有对应内容的主题不要硬凑。
每个主题包含：
- 重点事实
- 简短背景
- 来源编号

## 进展与重复报道
标出同一事件的进展关系、重复报道和仍不确定的信息。

## 值得继续关注
列出 3-5 项值得继续关注的事件、风险或后续进展。$tmpl$, true),
('群聊摘要', '适合群聊和频道消息，突出讨论主题、待办和风险。', 'markdown', $tmpl$# {{profile_name}}

## 重点摘要
- 输出 3-7 条重点摘要。
- 每条摘要必须标注来源编号，例如 [#1]。

## 主题归类
按讨论主题分组，每组包含：
- 主题名称
- 关键内容
- 相关来源编号

## 待办与风险
- 单独列出明确的待办、风险、问题和需要跟进的人或事项。
- 如果没有明确待办或风险，写"暂无明确待办或风险"。

## 低价值内容
用一句话说明被忽略的寒暄、表情、重复转发或无上下文短句。$tmpl$, true),
('待办提取', '适合从聊天记录中提取事项、负责人和风险。', 'markdown', $tmpl$# 待办提取

## 结论摘要
用 1 段话说明本窗口内最重要的事项。

## 明确待办
每项包含：
- 事项
- 负责人或相关人
- 截止时间或触发条件
- 来源编号

## 待确认问题
列出信息不足、需要追问或依赖外部确认的问题。

## 风险提示
列出可能影响执行的风险或阻塞点。$tmpl$, true);
-- +goose StatementEnd

-- +goose Down
ALTER TABLE ai_digest_profiles
    DROP COLUMN IF EXISTS output_template_id;
DROP TABLE IF EXISTS ai_digest_output_templates;
