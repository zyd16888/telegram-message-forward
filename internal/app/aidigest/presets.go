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

const financeVIPOutputTemplate = `# 财经 VIP 推荐候选池

## 本窗口概览
- 说明覆盖的栏目、时段、图片数量，以及无法识别或信息不足的部分。

## 被推荐标的
按股票合并同一窗口内的多来源观点，使用表格输出：
| 股票 | 代码/市场 | 推荐类型 | 题材与催化 | 图片原文证据 | 来源 | 置信度 |

规则：
- 推荐类型只能来自图片或消息明确表达，例如机构、游资、盘中宝；无法确认写“待确认”。
- 股票代码、价格和推荐主体无法确认时留空或写“待确认”，禁止依据常识补全。
- 同一股票存在不同来源时合并成一行，但证据和来源编号必须完整保留。

## 多源共识
仅列出被两个或以上独立来源明确提及的股票；没有则写“暂无多源共识”。

## 风险与待确认
列出图片模糊、代码冲突、时间不明、仅提及但未明确推荐等问题。

## 结构化数据
输出一个 JSON 代码块，必须符合以下结构且使用合法 JSON：
{
  "document_type": "institution_recommendation|hot_money_recommendation|intraday_brief|mixed|unknown",
  "publish_time": "ISO-8601 或 null",
  "market_session": "pre_market|intraday|after_market|unknown",
  "recommendations": [
    {
      "stock_name": "图片中的股票名",
      "stock_code": "图片中的股票代码或 null",
      "market": "A|HK|US|unknown",
      "recommender_type": "institution|hot_money|column|unknown",
      "themes": [],
      "catalysts": [],
      "price_mentions": [],
      "evidence": "图片或消息中的原始推荐语句",
      "source_refs": ["#1"],
      "confidence": 0.0
    }
  ]
}

## 说明
以上仅为对订阅资讯中明确推荐内容的提取与归纳，不构成投资建议。`

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
		{
			ID:          "finance_vip_candidates",
			Name:        "财经 VIP 图片候选池",
			Description: "适合盘中宝、机构和游资推荐图片，提取被推荐股票、原图证据与置信度。",
			PromptTemplate: `请分析以下财经资讯消息及其关联图片，形成“被推荐标的候选池”。

工作顺序：
1. 逐张读取图片中的栏目、发布时间、股票名称、股票代码、推荐主体、题材、催化和价格信息。
2. 判断内容属于盘中简报、机构推荐、游资推荐还是普通新闻；只提取明确出现的信息。
3. 区分“明确推荐”“仅提及”“多源共识”和“待确认”，不能把新闻中出现的股票自动视为推荐。
4. 同一 Telegram 相册组属于同一份材料，应结合多张图片理解，但每条结论仍需标注消息来源编号。
5. 禁止依据模型常识补股票代码、价格、机构名、行情或买卖建议。图片模糊或信息冲突时降低置信度并写入待确认。
6. 严格按照输出结构模板输出，末尾 JSON 必须可解析。

窗口：{{window_start}} 至 {{window_end}}
来源：{{source_list}}
消息数：{{message_count}}

输出结构模板：
{{output_template}}

{{messages}}

输出格式：{{output_format}}`,
			OutputFormat:   "markdown",
			OutputTemplate: financeVIPOutputTemplate,
			Schedule: domainaidigest.ScheduleConfig{
				Type:            "interval",
				IntervalMinutes: 30,
				Timezone:        "Asia/Shanghai",
			},
			Window:      domainaidigest.WindowConfig{Type: "since_last_run", DurationMinutes: 60},
			Dedupe:      domainaidigest.DedupeConfig{Enabled: true},
			ModelConfig: domainaidigest.ModelConfig{Temperature: 0.1, MaxTokens: 4000},
			Limits: domainaidigest.LimitsConfig{
				MaxMessagesPerRun: 80, MaxCharsPerMessage: 1600, MaxPromptChars: 120000,
			},
			Multimodal: domainaidigest.MultimodalConfig{
				Enabled: true, AllowExternalMedia: false, ImageDetail: "high", MaxImagesPerRun: 20,
				MaxImageBytes: 10 << 20, MaxTotalImageBytes: 60 << 20, FailureMode: "continue_text",
			},
		},
	}
}
