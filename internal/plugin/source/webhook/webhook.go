// Package webhook 实现通用 HTTP Webhook Source。
package webhook

import (
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	domainaccount "telegram-message-forward/internal/domain/account"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainsource "telegram-message-forward/internal/domain/source"
	pluginsource "telegram-message-forward/internal/plugin/source"
)

func init() {
	pluginsource.Register("webhook", func() (pluginsource.Plugin, error) {
		return New(), nil
	})
}

// Plugin 是通用 HTTP Webhook Source 插件。
type Plugin struct {
	log *slog.Logger

	mu      sync.Mutex
	runners map[int64]*runner
}

type runner struct {
	sourceID        int64
	status          string
	recentMessageAt *time.Time
	lastError       string
}

// New 创建 Webhook Source 插件。
func New() *Plugin {
	return &Plugin{log: slog.Default(), runners: map[int64]*runner{}}
}

var _ pluginsource.Plugin = (*Plugin)(nil)
var _ pluginsource.WebhookReceiver = (*Plugin)(nil)

func (p *Plugin) Name() string { return "webhook" }

func (p *Plugin) Capabilities() pluginsource.Capabilities {
	return pluginsource.Capabilities{SupportsSync: false, SupportsMedia: true, SupportsHistory: false}
}

func (p *Plugin) ValidateConfig(config map[string]any) error {
	if strings.TrimSpace(configString(config, "token")) == "" {
		return fmt.Errorf("webhook 缺少 token")
	}
	return nil
}

func (p *Plugin) Start(_ context.Context, _ *domainaccount.Account, src *domainsource.Source, _ pluginsource.Handler) error {
	if err := p.ValidateConfig(src.Config); err != nil {
		return err
	}
	p.mu.Lock()
	p.runners[src.ID] = &runner{sourceID: src.ID, status: "running"}
	p.mu.Unlock()
	return nil
}

func (p *Plugin) Stop(_ context.Context, src *domainsource.Source) error {
	p.mu.Lock()
	delete(p.runners, src.ID)
	p.mu.Unlock()
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

func (p *Plugin) Receive(_ context.Context, src *domainsource.Source, req pluginsource.WebhookRequest) (*domainmessage.NormalizedMessage, error) {
	payload := incomingPayload{}
	if len(req.Body) > 0 {
		if err := json.Unmarshal(req.Body, &payload); err != nil {
			payload.Text = strings.TrimSpace(string(req.Body))
		}
	}
	msg := normalize(src, payload, req.Body)
	p.recordReceived(src.ID)
	return msg, nil
}

func (p *Plugin) recordReceived(sourceID int64) {
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	r := p.runners[sourceID]
	if r == nil {
		r = &runner{sourceID: sourceID, status: "running"}
		p.runners[sourceID] = r
	}
	r.recentMessageAt = &now
	r.lastError = ""
}

type incomingPayload struct {
	MessageID      any                   `json:"message_id"`
	Text           string                `json:"text"`
	Format         string                `json:"format"`
	SenderID       int64                 `json:"sender_id"`
	SenderName     string                `json:"sender_name"`
	Sender         string                `json:"sender"`
	Timestamp      string                `json:"timestamp"`
	OriginalURL    string                `json:"original_url"`
	Links          []domainmessage.Link  `json:"links"`
	Media          []domainmessage.Media `json:"media"`
	MessageType    string                `json:"message_type"`
	SenderPeerType string                `json:"sender_peer_type"`
}

func normalize(src *domainsource.Source, payload incomingPayload, raw []byte) *domainmessage.NormalizedMessage {
	senderName := firstNonEmpty(payload.SenderName, payload.Sender)
	senderPeerType := firstNonEmpty(payload.SenderPeerType, "webhook")
	messageType := firstNonEmpty(payload.MessageType, "text")
	if len(payload.Media) > 0 {
		messageType = payload.Media[0].Type
	}
	return &domainmessage.NormalizedMessage{
		SourceID:          src.ID,
		ExternalMessageID: externalMessageID(payload, raw),
		MessageType:       messageType,
		SenderPeerType:    senderPeerType,
		SenderID:          payload.SenderID,
		SenderName:        senderName,
		Text:              payload.Text,
		Media:             payload.Media,
		Links:             payload.Links,
		OriginalURL:       payload.OriginalURL,
		RawPayload:        raw,
		SentAt:            parseTime(payload.Timestamp),
	}
}

func externalMessageID(payload incomingPayload, raw []byte) int64 {
	switch v := payload.MessageID.(type) {
	case float64:
		if v > 0 {
			return int64(v)
		}
	case string:
		v = strings.TrimSpace(v)
		if parsed, err := strconv.ParseInt(v, 10, 64); err == nil && parsed > 0 {
			return parsed
		}
		if v != "" {
			return stableID([]byte(v))
		}
	}
	if len(raw) > 0 {
		return stableID(raw)
	}
	return stableID([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
}

func stableID(data []byte) int64 {
	sum := sha1.Sum(data)
	return int64(binary.BigEndian.Uint64(sum[:8]) & 0x7fffffffffffffff)
}

func parseTime(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, time.RFC1123Z, time.RFC1123} {
		if t, err := time.Parse(layout, value); err == nil {
			return &t
		}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func configString(config map[string]any, key string) string {
	v, _ := config[key].(string)
	return strings.TrimSpace(v)
}
