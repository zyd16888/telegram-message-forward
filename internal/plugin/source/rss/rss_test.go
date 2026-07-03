package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
)

func TestParseRSSFeed(t *testing.T) {
	items, err := parseFeed([]byte(`<?xml version="1.0"?>
<rss version="2.0"><channel><item>
<guid>g1</guid><title>Hello</title><link>https://example.com/p/1</link>
<description><![CDATA[<p>World</p>]]></description>
<pubDate>Fri, 03 Jul 2026 12:00:00 +0800</pubDate>
<enclosure url="https://example.com/a.jpg" type="image/jpeg" length="12"/>
</item></channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "Hello" || items[0].Media == nil {
		t.Fatalf("解析结果不正确: %+v", items)
	}
	src := &domainsource.Source{ID: 7, Type: "rss", PeerType: "feed", PeerID: 1, Name: "feed"}
	msg := normalizeItem(src, items[0])
	if msg.MessageType != "image" || msg.Media[0].RemoteURL != "https://example.com/a.jpg" || msg.Text != "Hello\nWorld" {
		t.Fatalf("标准化结果不正确: %+v", msg)
	}
}

func TestParseAtomFeed(t *testing.T) {
	items, err := parseFeed([]byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom"><entry>
<id>tag:example.com,2026:1</id><title>Atom Hello</title>
<link href="https://example.com/a/1" rel="alternate"/>
<summary>Summary</summary><updated>2026-07-03T12:00:00+08:00</updated>
</entry></feed>`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Link != "https://example.com/a/1" || items[0].PublishedAt == nil {
		t.Fatalf("Atom 解析结果不正确: %+v", items)
	}
}

func TestPollOnceIngestsMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<rss><channel>
<item><guid>2</guid><title>Two</title><link>https://example.com/2</link></item>
<item><guid>1</guid><title>One</title><link>https://example.com/1</link></item>
</channel></rss>`))
	}))
	defer srv.Close()

	p := New()
	src := &domainsource.Source{ID: 3, Type: "rss", PeerType: "feed", PeerID: 9, Name: "RSS", Config: map[string]any{"feed_url": srv.URL}}
	var got []*domainmessage.NormalizedMessage
	err := p.pollOnce(context.Background(), src, func(ctx context.Context, msg *domainmessage.NormalizedMessage) error {
		got = append(got, msg)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Text != "Two" || got[1].OriginalURL != "https://example.com/1" {
		t.Fatalf("ingest 消息不正确: %+v", got)
	}
}

func TestValidateConfig(t *testing.T) {
	p := New()
	if err := p.ValidateConfig(map[string]any{}); err == nil {
		t.Fatal("缺少 feed_url 应报错")
	}
	if err := p.ValidateConfig(map[string]any{"feed_url": "https://example.com/feed.xml"}); err != nil {
		t.Fatalf("合法配置不应报错: %v", err)
	}
}

func TestRunnerStatus(t *testing.T) {
	p := New()
	src := &domainsource.Source{ID: 5, Type: "rss", Config: map[string]any{"feed_url": "https://example.com/feed.xml"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := p.Start(ctx, nil, src, func(ctx context.Context, msg *domainmessage.NormalizedMessage) error { return nil }); err != nil {
		t.Fatal(err)
	}
	statuses := p.RunnerStatuses()
	if len(statuses) != 1 || statuses[0].SourceIDs[0] != 5 || statuses[0].Status != "running" {
		t.Fatalf("运行状态不正确: %+v", statuses)
	}
	if err := p.Stop(context.Background(), src); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if len(p.RunnerStatuses()) != 0 {
		t.Fatalf("停止后不应再有状态: %+v", p.RunnerStatuses())
	}
}
