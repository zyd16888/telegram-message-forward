package chatarchive

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	domainarchive "telegram-message-forward/internal/domain/chatarchive"
)

// Format 是导出文件格式。
type Format string

const (
	// FormatJSONL 一行一条，最适合喂分析脚本或 LLM。
	FormatJSONL Format = "jsonl"
	// FormatCSV 适合 Excel / pandas。
	FormatCSV Format = "csv"
	// FormatMarkdown 按天分节，可读，也适合直接丢给 LLM 做分析。
	FormatMarkdown Format = "md"
)

// ParseFormat 解析并校验导出格式。
func ParseFormat(raw string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(raw))) {
	case FormatJSONL:
		return FormatJSONL, nil
	case FormatCSV:
		return FormatCSV, nil
	case FormatMarkdown, "markdown":
		return FormatMarkdown, nil
	case "":
		return FormatJSONL, nil
	default:
		return "", fmt.Errorf("%w: 不支持的导出格式 %q（可选 jsonl / csv / md）", ErrInvalidInput, raw)
	}
}

// ContentType 返回该格式的 HTTP Content-Type。
func (f Format) ContentType() string {
	switch f {
	case FormatCSV:
		return "text/csv; charset=utf-8"
	case FormatMarkdown:
		return "text/markdown; charset=utf-8"
	default:
		return "application/x-ndjson; charset=utf-8"
	}
}

// renderBatch 是每次从库里取出的条数。流式读取，避免几万条全量载入内存。
const renderBatch = 500

// exportRow 是 JSONL 的一行。字段名固定且自解释，便于下游脚本消费。
type exportRow struct {
	MessageID     int64                    `json:"message_id"`
	Date          string                   `json:"date,omitempty"`
	EditDate      string                   `json:"edit_date,omitempty"`
	Direction     string                   `json:"direction"`
	SenderID      int64                    `json:"sender_id,omitempty"`
	SenderName    string                   `json:"sender_name,omitempty"`
	SenderHandle  string                   `json:"sender_username,omitempty"`
	ReplyTo       int64                    `json:"reply_to_message_id,omitempty"`
	GroupedID     int64                    `json:"grouped_id,omitempty"`
	Type          string                   `json:"type"`
	Text          string                   `json:"text"`
	Entities      []domainarchive.Entity   `json:"entities,omitempty"`
	Media         []domainarchive.Media    `json:"media,omitempty"`
	Forward       *domainarchive.Forward   `json:"forward,omitempty"`
	Reactions     []domainarchive.Reaction `json:"reactions,omitempty"`
	ServiceAction string                   `json:"service_action,omitempty"`
	Views         int                      `json:"views,omitempty"`
}

// Render 按格式流式渲染归档消息。
//
// 全程流式：按主键分批读库、边读边写。几万条消息的 JSONL 有几十 MB，
// 先在内存里拼完整个文件会直接把服务打爆。
func (s *Service) Render(ctx context.Context, format Format, q domainarchive.ExportQuery, w io.Writer) error {
	if q.ArchiveID <= 0 {
		return ErrInvalidInput
	}
	if q.From != nil && q.To != nil && !q.From.Before(*q.To) {
		return fmt.Errorf("%w: 起始时间必须早于结束时间", ErrInvalidInput)
	}
	q.Limit = renderBatch

	buf := bufio.NewWriterSize(w, 64*1024)
	var (
		flushTail func() error
		writeRow  func(*domainarchive.Message) error
	)

	switch format {
	case FormatCSV:
		writeRow, flushTail = s.csvRenderer(buf)
	case FormatMarkdown:
		writeRow, flushTail = s.markdownRenderer(ctx, buf, q.ArchiveID)
	default:
		writeRow, flushTail = s.jsonlRenderer(buf)
	}

	afterID := int64(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		q.AfterID = afterID
		page, err := s.messages.StreamPage(ctx, q)
		if err != nil {
			return err
		}
		if len(page) == 0 {
			break
		}
		for _, m := range page {
			if err := writeRow(m); err != nil {
				return err
			}
			afterID = m.ID
		}
		if len(page) < renderBatch {
			break
		}
	}

	if err := flushTail(); err != nil {
		return err
	}
	return buf.Flush()
}

func (s *Service) jsonlRenderer(w io.Writer) (func(*domainarchive.Message) error, func() error) {
	enc := json.NewEncoder(w)
	// 不转义 <、>、& ：聊天正文里这些字符很常见，转义后可读性很差。
	enc.SetEscapeHTML(false)
	return func(m *domainarchive.Message) error {
			return enc.Encode(toExportRow(m))
		}, func() error {
			return nil
		}
}

func (s *Service) csvRenderer(w io.Writer) (func(*domainarchive.Message) error, func() error) {
	// UTF-8 BOM：没有它 Excel 打开中文 CSV 会乱码。
	_, _ = io.WriteString(w, "\uFEFF")
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{
		"message_id", "date", "direction", "sender_id", "sender_name",
		"reply_to_message_id", "type", "text", "media", "service_action",
	})
	return func(m *domainarchive.Message) error {
			return cw.Write([]string{
				strconv.FormatInt(m.MessageID, 10),
				formatTime(m.Date),
				direction(m),
				strconv.FormatInt(m.SenderID, 10),
				m.SenderName,
				optionalID(m.ReplyToMessageID),
				m.MessageType,
				m.Text,
				mediaSummary(m.Media),
				m.ServiceAction,
			})
		}, func() error {
			cw.Flush()
			return cw.Error()
		}
}

func (s *Service) markdownRenderer(ctx context.Context, w io.Writer, archiveID int64) (func(*domainarchive.Message) error, func() error) {
	header := "# 聊天记录归档\n"
	if archive, err := s.archives.GetByID(ctx, archiveID); err == nil && archive != nil {
		name := archive.PeerName
		if name == "" {
			name = fmt.Sprintf("%s:%d", archive.PeerType, archive.PeerID)
		}
		header = fmt.Sprintf("# 聊天记录归档 · %s\n", name)
	}
	_, _ = io.WriteString(w, header)

	currentDay := ""
	return func(m *domainarchive.Message) error {
			day := ""
			if m.Date != nil {
				day = m.Date.Format("2006-01-02")
			}
			if day != currentDay {
				currentDay = day
				title := day
				if title == "" {
					title = "未知日期"
				}
				if _, err := fmt.Fprintf(w, "\n## %s\n\n", title); err != nil {
					return err
				}
			}

			clock := ""
			if m.Date != nil {
				clock = m.Date.Format("15:04:05")
			}
			if m.ServiceAction != "" {
				_, err := fmt.Fprintf(w, "- `%s` *[系统] %s*\n", clock, m.ServiceAction)
				return err
			}

			sender := m.SenderName
			if sender == "" {
				sender = strconv.FormatInt(m.SenderID, 10)
			}
			arrow := "←"
			if m.Out {
				arrow = "→"
			}
			var b strings.Builder
			fmt.Fprintf(&b, "- `%s` %s **%s**", clock, arrow, sender)
			if m.ReplyToMessageID > 0 {
				fmt.Fprintf(&b, " ↩️#%d", m.ReplyToMessageID)
			}
			b.WriteString("：")
			if text := strings.TrimSpace(m.Text); text != "" {
				// 多行正文缩进为子项，避免破坏列表结构。
				b.WriteString(strings.ReplaceAll(text, "\n", "\n  "))
			}
			if summary := mediaSummary(m.Media); summary != "" {
				fmt.Fprintf(&b, " *[%s]*", summary)
			}
			b.WriteString("\n")
			_, err := io.WriteString(w, b.String())
			return err
		}, func() error {
			return nil
		}
}

func toExportRow(m *domainarchive.Message) exportRow {
	row := exportRow{
		MessageID:     m.MessageID,
		Date:          formatTime(m.Date),
		EditDate:      formatTime(m.EditDate),
		Direction:     direction(m),
		SenderID:      m.SenderID,
		SenderName:    m.SenderName,
		SenderHandle:  m.SenderUsername,
		ReplyTo:       m.ReplyToMessageID,
		Type:          m.MessageType,
		Text:          m.Text,
		Entities:      m.Entities,
		Media:         m.Media,
		Forward:       m.Forward,
		Reactions:     m.Reactions,
		ServiceAction: m.ServiceAction,
		Views:         m.Views,
	}
	if m.GroupedID != nil {
		row.GroupedID = *m.GroupedID
	}
	return row
}

// direction 用 in/out 表达方向，比裸布尔在 CSV / LLM 里可读。
func direction(m *domainarchive.Message) string {
	if m.Out {
		return "out"
	}
	return "in"
}

func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func optionalID(id int64) string {
	if id <= 0 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}

// mediaSummary 把媒体压成一行可读描述，供 CSV / Markdown 使用。
func mediaSummary(media []domainarchive.Media) string {
	if len(media) == 0 {
		return ""
	}
	parts := make([]string, 0, len(media))
	for _, m := range media {
		item := m.Type
		if m.FileName != "" {
			item += ":" + m.FileName
		}
		if !m.Downloaded {
			item += "(未下载)"
		}
		parts = append(parts, item)
	}
	return strings.Join(parts, "; ")
}

// ExportFileName 生成下载文件名。
func ExportFileName(archive *domainarchive.Archive, format Format) string {
	name := "chat"
	if archive != nil {
		switch {
		case archive.PeerUsername != "":
			name = archive.PeerUsername
		case archive.PeerName != "":
			name = archive.PeerName
		default:
			name = fmt.Sprintf("%s-%d", archive.PeerType, archive.PeerID)
		}
	}
	return fmt.Sprintf("%s-archive.%s", sanitizeFileName(name), format)
}

// sanitizeFileName 去掉路径分隔符与控制字符，避免 Content-Disposition 被注入。
func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r < 0x20, r == 0x7f:
			continue
		case r == '/', r == '\\', r == '"', r == ':', r == '*', r == '?', r == '<', r == '>', r == '|':
			b.WriteRune('-')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "chat"
	}
	if len([]rune(out)) > 60 {
		out = string([]rune(out)[:60])
	}
	return out
}
