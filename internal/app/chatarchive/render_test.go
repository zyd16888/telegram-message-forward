package chatarchive

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	domainarchive "telegram-message-forward/internal/domain/chatarchive"
)

func sampleMessages() []*domainarchive.Message {
	t1 := time.Date(2026, 4, 1, 9, 30, 0, 0, time.UTC)
	t2 := time.Date(2026, 4, 1, 9, 31, 0, 0, time.UTC)
	t3 := time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC)
	return []*domainarchive.Message{
		{ID: 1, MessageID: 101, Date: &t1, Out: false, SenderID: 7, SenderName: "阿达", MessageType: "text", Text: "在吗"},
		{ID: 2, MessageID: 102, Date: &t2, Out: true, SenderID: 1, SenderName: "我", MessageType: "text",
			Text: "在的 <你好> & 100%", ReplyToMessageID: 101},
		{ID: 3, MessageID: 103, Date: &t3, Out: false, SenderID: 7, SenderName: "阿达", MessageType: "photo",
			Media: []domainarchive.Media{{Type: "photo", FileName: "a.jpg"}}},
		{ID: 4, MessageID: 104, Date: &t3, ServiceAction: "chat_add_user", MessageType: "service"},
	}
}

func renderTo(t *testing.T, format Format, includeService bool) (string, *stubMessageRepo) {
	t.Helper()
	repo := &stubMessageRepo{all: sampleMessages()}
	svc := testService(repo, &domainarchive.Archive{ID: 5, PeerName: "阿达", PeerType: domainarchive.PeerUser, PeerID: 7})
	var buf bytes.Buffer
	err := svc.Render(context.Background(), format, domainarchive.ExportQuery{
		ArchiveID: 5, IncludeService: includeService,
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	return buf.String(), repo
}

func TestRenderJSONL(t *testing.T) {
	out, _ := renderTo(t, FormatJSONL, false)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("应输出 3 行（服务消息被过滤），实际 %d：%q", len(lines), out)
	}

	var first exportRow
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("JSONL 首行不是合法 JSON: %v", err)
	}
	if first.MessageID != 101 || first.Direction != "in" || first.SenderName != "阿达" {
		t.Fatalf("首行内容错误: %+v", first)
	}

	var second exportRow
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatal(err)
	}
	if second.Direction != "out" || second.ReplyTo != 101 {
		t.Fatalf("方向/回复链丢失: %+v", second)
	}
	// 聊天正文里 < > & 很常见，被 encoding/json 默认转义成 \uXXXX 后可读性很差。
	if !strings.Contains(lines[1], "<你好> & 100%") {
		t.Fatalf("正文应原样保留: %s", lines[1])
	}
	esc := string([]byte{'\\', 'u', '0', '0'})
	for _, suffix := range []string{"3c", "3e", "26"} {
		if strings.Contains(lines[1], esc+suffix) {
			t.Fatalf("正文中的 <>& 不应被 HTML 转义（出现 %s）: %s", esc+suffix, lines[1])
		}
	}
}

func TestRenderJSONLIncludesService(t *testing.T) {
	out, _ := renderTo(t, FormatJSONL, true)
	if !strings.Contains(out, "chat_add_user") {
		t.Fatalf("include_service 时服务消息应保留: %s", out)
	}
}

func TestRenderCSV(t *testing.T) {
	out, _ := renderTo(t, FormatCSV, false)
	// Excel 打开中文 CSV 需要 BOM，否则乱码。
	if !strings.HasPrefix(out, "\uFEFF") {
		t.Fatal("CSV 应以 UTF-8 BOM 开头")
	}

	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(out, "\uFEFF"))).ReadAll()
	if err != nil {
		t.Fatalf("CSV 不合法: %v", err)
	}
	if len(rows) != 4 {
		t.Fatalf("应为表头 + 3 行，实际 %d 行", len(rows))
	}
	if rows[0][0] != "message_id" || rows[0][5] != "reply_to_message_id" {
		t.Fatalf("表头错误: %v", rows[0])
	}
	if rows[2][2] != "out" || rows[2][5] != "101" {
		t.Fatalf("方向/回复链列错误: %v", rows[2])
	}
	if rows[3][8] != "photo:a.jpg(未下载)" {
		t.Fatalf("媒体摘要错误: %v", rows[3])
	}
}

func TestRenderMarkdownGroupsByDay(t *testing.T) {
	out, _ := renderTo(t, FormatMarkdown, true)
	if !strings.Contains(out, "# 聊天记录归档 · 阿达") {
		t.Fatalf("缺少标题: %s", out)
	}
	if strings.Count(out, "## 2026-04-01") != 1 || strings.Count(out, "## 2026-04-02") != 1 {
		t.Fatalf("应按天分节且不重复: %s", out)
	}
	if !strings.Contains(out, "↩️#101") {
		t.Fatalf("回复关系应可见: %s", out)
	}
	if !strings.Contains(out, "[系统] chat_add_user") {
		t.Fatalf("服务消息应标注: %s", out)
	}
	// 出/入方向要能一眼看出。
	if !strings.Contains(out, "→ **我**") || !strings.Contains(out, "← **阿达**") {
		t.Fatalf("方向标记缺失: %s", out)
	}
}

// 渲染必须分批读库，不能一次把整个归档载入内存。
func TestRenderStreamsInBatches(t *testing.T) {
	msgs := make([]*domainarchive.Message, 0, renderBatch+10)
	day := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= renderBatch+10; i++ {
		d := day.Add(time.Duration(i) * time.Second)
		msgs = append(msgs, &domainarchive.Message{
			ID: int64(i), MessageID: int64(1000 + i), Date: &d, MessageType: "text", Text: "x",
		})
	}
	repo := &stubMessageRepo{all: msgs}
	svc := testService(repo, &domainarchive.Archive{ID: 1})

	var buf bytes.Buffer
	if err := svc.Render(context.Background(), FormatJSONL, domainarchive.ExportQuery{ArchiveID: 1}, &buf); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(strings.TrimRight(buf.String(), "\n"), "\n") + 1; got != renderBatch+10 {
		t.Fatalf("输出行数 = %d, want %d", got, renderBatch+10)
	}
	if repo.calls < 2 {
		t.Fatalf("应分批读库，实际只调用 %d 次", repo.calls)
	}
}

func TestParseFormat(t *testing.T) {
	for raw, want := range map[string]Format{
		"": FormatJSONL, "jsonl": FormatJSONL, "csv": FormatCSV,
		"md": FormatMarkdown, "markdown": FormatMarkdown, "  CSV  ": FormatCSV,
	} {
		got, err := ParseFormat(raw)
		if err != nil || got != want {
			t.Fatalf("ParseFormat(%q) = %v, %v; want %v", raw, got, err, want)
		}
	}
	if _, err := ParseFormat("pdf"); err == nil {
		t.Fatal("不支持的格式应报错")
	}
}

func TestRenderRejectsBadRange(t *testing.T) {
	svc := testService(&stubMessageRepo{}, &domainarchive.Archive{ID: 1})
	from := time.Date(2026, 5, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	err := svc.Render(context.Background(), FormatJSONL,
		domainarchive.ExportQuery{ArchiveID: 1, From: &from, To: &to}, io.Discard)
	if err == nil {
		t.Fatal("起始晚于结束应报错")
	}
}

// 文件名会进 Content-Disposition，必须过滤路径分隔符与引号。
func TestExportFileNameSanitized(t *testing.T) {
	got := ExportFileName(&domainarchive.Archive{PeerName: `a/b"c:d`}, FormatCSV)
	if strings.ContainsAny(got[:len(got)-4], `/\":`) {
		t.Fatalf("文件名未净化: %s", got)
	}
	if !strings.HasSuffix(got, ".csv") {
		t.Fatalf("扩展名错误: %s", got)
	}
	if got := ExportFileName(nil, FormatJSONL); got != "chat-archive.jsonl" {
		t.Fatalf("空归档文件名 = %s", got)
	}
}
