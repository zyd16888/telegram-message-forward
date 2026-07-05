package aidigest

import domainaidigest "telegram-message-forward/internal/domain/aidigest"

func defaultPresets() []domainaidigest.Preset {
	return []domainaidigest.Preset{
		{
			ID:          "group_digest",
			Name:        "群消息归纳整理",
			Description: "适合群聊、频道和工作流消息的窗口摘要，突出结论、待办和来源编号。",
			PromptTemplate: `请把以下群消息整理成一份可直接阅读的简报。

目标：
1. 先给出 3-7 条重点摘要，每条都标注来源编号。
2. 按主题归类列出讨论内容，合并重复信息。
3. 单独列出明确的待办、风险、问题和需要跟进的人或事项。
4. 忽略寒暄、表情、无上下文短句和重复转发。
5. 只基于输入消息，不补充外部事实。

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

{{messages}}

输出格式：{{output_format}}`,
			OutputFormat: "markdown",
			Schedule: domainaidigest.ScheduleConfig{
				Type:            "interval",
				IntervalMinutes: 60,
				Timezone:        "Asia/Shanghai",
			},
			Window:      domainaidigest.WindowConfig{Type: "last_duration", DurationMinutes: 60},
			Dedupe:      domainaidigest.DedupeConfig{Enabled: true},
			ModelConfig: domainaidigest.ModelConfig{Temperature: 0.2, MaxTokens: 1200},
			Limits:      domainaidigest.LimitsConfig{MaxMessagesPerRun: 80, MaxCharsPerMessage: 1200, MaxPromptChars: 36000},
		},
		{
			ID:          "daily_news",
			Name:        "今日新闻总结",
			Description: "适合新闻频道或资讯源的每日汇总，按主题和重要性组织。",
			PromptTemplate: `请把以下资讯消息整理成“今日新闻总结”。

要求：
1. 按主题分组，例如国际、国内、科技、财经、行业动态；没有对应内容的主题不要硬凑。
2. 每个主题先列重点，再列简短背景，所有关键事实都标注来源编号。
3. 标出重复报道、同一事件的进展关系，以及信息仍不确定的地方。
4. 不要补充输入之外的事实，不要给投资、医疗、法律等建议。
5. 末尾给出“值得继续关注”的 3-5 项。

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

{{messages}}

输出格式：{{output_format}}`,
			OutputFormat: "markdown",
			Schedule: domainaidigest.ScheduleConfig{
				Type:     "daily",
				Time:     "09:00",
				Timezone: "Asia/Shanghai",
			},
			Window:      domainaidigest.WindowConfig{Type: "last_duration", DurationMinutes: 1440},
			Dedupe:      domainaidigest.DedupeConfig{Enabled: true},
			ModelConfig: domainaidigest.ModelConfig{Temperature: 0.2, MaxTokens: 1800},
			Limits:      domainaidigest.LimitsConfig{MaxMessagesPerRun: 120, MaxCharsPerMessage: 1000, MaxPromptChars: 50000},
		},
	}
}
