package aidigest

import domainaidigest "telegram-message-forward/internal/domain/aidigest"

const groupDigestOutputTemplate = `# {{profile_name}}

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
- 如果没有明确待办或风险，写“暂无明确待办或风险”。

## 低价值内容
用一句话说明被忽略的寒暄、表情、重复转发或无上下文短句。`

const dailyNewsOutputTemplate = `# 今日新闻总结

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
列出 3-5 项值得继续关注的事件、风险或后续进展。`

func defaultPresets() []domainaidigest.Preset {
	return []domainaidigest.Preset{
		{
			ID:          "group_digest",
			Name:        "群消息归纳整理",
			Description: "适合群聊、频道和工作流消息的窗口摘要，突出结论、待办和来源编号。",
			PromptTemplate: `请把以下群消息整理成一份可直接阅读的简报。

目标：
1. 忽略寒暄、表情、无上下文短句和重复转发。
2. 合并重复信息，不要重复罗列同一件事。
3. 只基于输入消息，不补充外部事实。
4. 严格按照输出结构模板输出。

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

输出结构模板：
{{output_template}}

{{messages}}

输出格式：{{output_format}}`,
			OutputFormat:   "markdown",
			OutputTemplate: groupDigestOutputTemplate,
			Schedule: domainaidigest.ScheduleConfig{
				Type:            "interval",
				IntervalMinutes: 60,
				Timezone:        "Asia/Shanghai",
			},
			Window:      domainaidigest.WindowConfig{Type: "last_duration", DurationMinutes: 60},
			Dedupe:      domainaidigest.DedupeConfig{Enabled: true},
			ModelConfig: domainaidigest.ModelConfig{Temperature: 0.2},
			Limits:      domainaidigest.LimitsConfig{MaxMessagesPerRun: 80, MaxCharsPerMessage: 1200},
		},
		{
			ID:          "daily_news",
			Name:        "今日新闻总结",
			Description: "适合新闻频道或资讯源的每日汇总，按主题和重要性组织。",
			PromptTemplate: `请把以下资讯消息整理成“今日新闻总结”。

要求：
1. 所有关键事实都标注来源编号。
2. 不要补充输入之外的事实，不要给投资、医疗、法律等建议。
3. 严格按照输出结构模板输出。

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

输出结构模板：
{{output_template}}

{{messages}}

输出格式：{{output_format}}`,
			OutputFormat:   "markdown",
			OutputTemplate: dailyNewsOutputTemplate,
			Schedule: domainaidigest.ScheduleConfig{
				Type:     "daily",
				Time:     "09:00",
				Timezone: "Asia/Shanghai",
			},
			Window:      domainaidigest.WindowConfig{Type: "last_duration", DurationMinutes: 1440},
			Dedupe:      domainaidigest.DedupeConfig{Enabled: true},
			ModelConfig: domainaidigest.ModelConfig{Temperature: 0.2},
			Limits:      domainaidigest.LimitsConfig{MaxMessagesPerRun: 120, MaxCharsPerMessage: 1000},
		},
	}
}
