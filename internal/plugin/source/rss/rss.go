// Package rss 实现 RSS/Atom 订阅 Source。
package rss

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

const (
	defaultPollInterval = 5 * time.Minute
	defaultMaxItems     = 20
)

func init() {
	pluginsource.Register("rss", func() (pluginsource.Plugin, error) {
		return New(), nil
	})
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Plugin 是 RSS/Atom Source 插件。
type Plugin struct {
	client httpDoer
	log    *slog.Logger

	mu      sync.Mutex
	runners map[int64]*runner
}

type runner struct {
	sourceID        int64
	cancel          context.CancelFunc
	status          string
	recentMessageAt *time.Time
	lastError       string
}

// New 创建 RSS Source 插件。
func New() *Plugin {
	return &Plugin{
		client:  &http.Client{Timeout: 30 * time.Second},
		log:     slog.Default(),
		runners: map[int64]*runner{},
	}
}

var _ pluginsource.Plugin = (*Plugin)(nil)

func (p *Plugin) Name() string { return "rss" }

func (p *Plugin) Capabilities() pluginsource.Capabilities {
	return pluginsource.Capabilities{SupportsSync: false, SupportsMedia: true, SupportsHistory: false}
}

func (p *Plugin) ValidateConfig(config map[string]any) error {
	if strings.TrimSpace(configString(config, "feed_url")) == "" {
		return fmt.Errorf("rss 缺少 feed_url")
	}
	return nil
}

func (p *Plugin) Start(parent context.Context, _ *domainaccount.Account, src *domainsource.Source, handler pluginsource.Handler) error {
	if err := p.ValidateConfig(src.Config); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(parent)
	r := &runner{sourceID: src.ID, cancel: cancel, status: "running"}

	p.mu.Lock()
	if old := p.runners[src.ID]; old != nil {
		old.cancel()
	}
	p.runners[src.ID] = r
	p.mu.Unlock()

	go p.pollLoop(runCtx, src, handler, r)
	return nil
}

func (p *Plugin) Stop(_ context.Context, src *domainsource.Source) error {
	p.mu.Lock()
	r := p.runners[src.ID]
	delete(p.runners, src.ID)
	p.mu.Unlock()
	if r != nil {
		r.cancel()
	}
	return nil
}

func (p *Plugin) SyncSources(context.Context, *domainaccount.Account) ([]pluginsource.SyncedPeer, error) {
	return nil, nil
}

func (p *Plugin) RunnerStatuses() []pluginsource.RunnerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]pluginsource.RunnerStatus, 0, len(p.runners))
	for _, r := range p.runners {
		out = append(out, pluginsource.RunnerStatus{
			SourceIDs:         []int64{r.sourceID},
			Status:            r.status,
			SubscriptionCount: 1,
			RecentMessageAt:   r.recentMessageAt,
			LastError:         r.lastError,
		})
	}
	return out
}

func (p *Plugin) pollLoop(ctx context.Context, src *domainsource.Source, handler pluginsource.Handler, r *runner) {
	interval := pollInterval(src.Config)
	p.pollOnceAndRecord(ctx, src, handler, r)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.pollOnceAndRecord(ctx, src, handler, r)
		}
	}
}

func (p *Plugin) pollOnceAndRecord(ctx context.Context, src *domainsource.Source, handler pluginsource.Handler, r *runner) {
	err := p.pollOnce(ctx, src, handler)
	p.mu.Lock()
	defer p.mu.Unlock()
	if current := p.runners[src.ID]; current != r {
		return
	}
	if err != nil {
		r.lastError = err.Error()
		p.log.Warn("RSS source 拉取失败", "source", src.ID, "err", err)
		return
	}
	now := time.Now()
	r.recentMessageAt = &now
	r.lastError = ""
}

func (p *Plugin) pollOnce(ctx context.Context, src *domainsource.Source, handler pluginsource.Handler) error {
	items, err := p.fetch(ctx, src.Config)
	if err != nil {
		return err
	}
	sort.SliceStable(items, func(i, j int) bool {
		return itemTime(items[i]).Before(itemTime(items[j]))
	})
	for _, item := range items {
		msg := normalizeItem(src, item)
		if err := handler(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

func (p *Plugin) fetch(ctx context.Context, config map[string]any) ([]feedItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, configString(config, "feed_url"), nil)
	if err != nil {
		return nil, fmt.Errorf("构建 RSS 请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "telegram-message-forward/rss-source")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("拉取 RSS 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("RSS 返回非 2xx 状态: %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("读取 RSS 响应失败: %w", err)
	}
	items, err := parseFeed(data)
	if err != nil {
		return nil, err
	}
	maxItems := configInt(config, "max_items", defaultMaxItems)
	if maxItems > 0 && len(items) > maxItems {
		items = items[len(items)-maxItems:]
	}
	return items, nil
}

type feed struct {
	Channel rssChannel  `xml:"channel"`
	Entries []atomEntry `xml:"entry"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	GUID        string    `xml:"guid"`
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate"`
	Enclosure   enclosure `xml:"enclosure"`
	Content     string    `xml:"encoded"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Links     []atomLink `xml:"link"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

type feedItem struct {
	ID          string
	Title       string
	Link        string
	Summary     string
	PublishedAt *time.Time
	Media       *enclosure
}

func parseFeed(data []byte) ([]feedItem, error) {
	var f feed
	if err := xml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("解析 RSS/Atom 失败: %w", err)
	}
	items := make([]feedItem, 0, len(f.Channel.Items)+len(f.Entries))
	for _, item := range f.Channel.Items {
		items = append(items, feedItem{
			ID:          firstNonEmpty(item.GUID, item.Link, item.Title),
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			Summary:     firstNonEmpty(item.Description, item.Content),
			PublishedAt: parseTime(item.PubDate),
			Media:       enclosureOrNil(item.Enclosure),
		})
	}
	for _, entry := range f.Entries {
		items = append(items, feedItem{
			ID:          firstNonEmpty(entry.ID, atomEntryLink(entry), entry.Title),
			Title:       strings.TrimSpace(entry.Title),
			Link:        atomEntryLink(entry),
			Summary:     firstNonEmpty(entry.Summary, entry.Content),
			PublishedAt: parseTime(firstNonEmpty(entry.Published, entry.Updated)),
			Media:       nil,
		})
	}
	return items, nil
}

func normalizeItem(src *domainsource.Source, item feedItem) *domainmessage.NormalizedMessage {
	text := strings.TrimSpace(item.Title)
	summary := strings.TrimSpace(stripHTML(item.Summary))
	if summary != "" {
		if text != "" {
			text += "\n"
		}
		text += summary
	}
	raw, _ := json.Marshal(item)
	msg := &domainmessage.NormalizedMessage{
		SourceID:          src.ID,
		ExternalMessageID: stableMessageID(item),
		MessageType:       "text",
		SenderPeerType:    "rss",
		SenderID:          src.PeerID,
		SenderName:        src.Name,
		Text:              text,
		OriginalURL:       item.Link,
		RawPayload:        raw,
		SentAt:            item.PublishedAt,
	}
	if item.Link != "" {
		msg.Links = []domainmessage.Link{{URL: item.Link, Title: item.Title}}
	}
	if media := mediaFromEnclosure(item.Media, item.Title); media != nil {
		msg.Media = []domainmessage.Media{*media}
		msg.MessageType = media.Type
	}
	return msg
}

func mediaFromEnclosure(e *enclosure, caption string) *domainmessage.Media {
	if e == nil || e.URL == "" {
		return nil
	}
	mediaType := "document"
	switch {
	case strings.HasPrefix(e.Type, "image/"):
		mediaType = "image"
	case strings.HasPrefix(e.Type, "audio/"):
		mediaType = "audio"
	case strings.HasPrefix(e.Type, "video/"):
		mediaType = "video"
	}
	return &domainmessage.Media{
		Type:      mediaType,
		RemoteURL: e.URL,
		MimeType:  e.Type,
		Size:      parseInt64(e.Length),
		Caption:   caption,
	}
}

func itemTime(item feedItem) time.Time {
	if item.PublishedAt != nil {
		return *item.PublishedAt
	}
	return time.Time{}
}

func stableMessageID(item feedItem) int64 {
	key := firstNonEmpty(item.ID, item.Link, item.Title)
	if item.PublishedAt != nil {
		key += "|" + item.PublishedAt.UTC().Format(time.RFC3339Nano)
	}
	sum := sha1.Sum([]byte(key))
	return int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
}

func pollInterval(config map[string]any) time.Duration {
	seconds := configInt(config, "poll_interval_seconds", int(defaultPollInterval.Seconds()))
	if seconds < 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func parseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	layouts := []string{time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano, "Mon, 02 Jan 2006 15:04:05 -0700"}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

func atomEntryLink(entry atomEntry) string {
	for _, link := range entry.Links {
		if link.Rel == "" || link.Rel == "alternate" {
			return strings.TrimSpace(link.Href)
		}
	}
	if len(entry.Links) > 0 {
		return strings.TrimSpace(entry.Links[0].Href)
	}
	return ""
}

func enclosureOrNil(e enclosure) *enclosure {
	if e.URL == "" {
		return nil
	}
	return &e
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stripHTML(value string) string {
	var b strings.Builder
	inTag := false
	for _, r := range value {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func parseInt64(value string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	return n
}

func configString(config map[string]any, key string) string {
	v, _ := config[key].(string)
	return strings.TrimSpace(v)
}

func configInt(config map[string]any, key string, fallback int) int {
	switch v := config[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}
