// Package ingest 编排「标准消息 → 落库 → 规则匹配 → 生成投递任务」主链路。
//
// 它是 Source 插件与投递队列之间的入口，不解析 Telegram 原始消息，
// 只接收已标准化的 NormalizedMessage。
package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"telegram-message-forward/internal/dispatch"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/flowengine"
	"telegram-message-forward/internal/infra/clock"
	"telegram-message-forward/internal/infra/mediastore"
	"telegram-message-forward/internal/ruleengine"
)

// Service 是消息 ingest 应用服务。
type Service struct {
	messages   domainmessage.Repository
	rules      domainrule.Repository
	flows      domainflow.Repository
	engine     *ruleengine.Engine
	flowEngine *flowengine.Engine
	flowMode   string
	queue      *dispatch.Queue
	clock      clock.Clock
	log        *slog.Logger
	media      mediastore.Store
}

// NewService 创建 ingest 服务。
func NewService(
	messages domainmessage.Repository,
	rules domainrule.Repository,
	engine *ruleengine.Engine,
	queue *dispatch.Queue,
	clk clock.Clock,
	log *slog.Logger,
) *Service {
	return &Service{
		messages: messages, rules: rules, engine: engine,
		queue: queue, clock: clk, log: log,
	}
}

// UseFlowEngine 注入 Flow 图引擎。mode 支持 off、shadow、primary。
func (s *Service) UseFlowEngine(flows domainflow.Repository, engine *flowengine.Engine, mode string) *Service {
	s.flows = flows
	s.flowEngine = engine
	s.flowMode = strings.ToLower(strings.TrimSpace(mode))
	return s
}

// UseMediaStore 注入媒体存储：Source 下载到临时目录的媒体在落库前收编到存储层。
func (s *Service) UseMediaStore(store mediastore.Store) *Service {
	s.media = store
	return s
}

// Ingest 处理一条标准化消息：幂等落库 → 匹配规则 → 生成投递任务。
//
// 落库使用 (source_id, external_message_id) 幂等约束；重复消息不会重复投递。
func (s *Service) Ingest(ctx context.Context, msg *domainmessage.NormalizedMessage) error {
	if msg.ReceivedAt.IsZero() {
		msg.ReceivedAt = s.clock.Now()
	}
	if msg.MessageType == "" {
		msg.MessageType = "text"
	}

	s.persistMedia(ctx, msg)

	if err := s.messages.Create(ctx, msg); err != nil {
		return err
	}

	matches, err := s.evaluate(ctx, msg)
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return nil
	}

	if err := s.queue.Enqueue(ctx, msg, matches); err != nil {
		return err
	}

	s.log.Info("消息已入队投递",
		"message_id", msg.ID,
		"source_id", msg.SourceID,
		"matched_rules", len(matches),
	)
	return nil
}

func (s *Service) evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage) ([]ruleengine.Match, error) {
	switch s.flowMode {
	case "primary":
		return s.evaluateFlows(ctx, msg)
	case "shadow":
		matches, err := s.evaluateRules(ctx, msg)
		if err != nil {
			return nil, err
		}
		s.shadowCompare(ctx, msg, matches)
		return matches, nil
	default:
		return s.evaluateRules(ctx, msg)
	}
}

func (s *Service) evaluateRules(ctx context.Context, msg *domainmessage.NormalizedMessage) ([]ruleengine.Match, error) {
	rules, err := s.rules.ListEnabledBySource(ctx, msg.SourceID)
	if err != nil || len(rules) == 0 {
		return nil, err
	}
	return s.engine.Evaluate(ctx, msg, rules)
}

func (s *Service) evaluateFlows(ctx context.Context, msg *domainmessage.NormalizedMessage) ([]ruleengine.Match, error) {
	if s.flows == nil || s.flowEngine == nil {
		return nil, nil
	}
	flows, err := s.flows.ListEnabledBySource(ctx, msg.SourceID)
	if err != nil || len(flows) == 0 {
		return nil, err
	}
	return s.flowEngine.Evaluate(ctx, msg, flows)
}

func (s *Service) evaluateMigratedFlows(ctx context.Context, msg *domainmessage.NormalizedMessage) ([]ruleengine.Match, error) {
	if s.flows == nil || s.flowEngine == nil {
		return nil, nil
	}
	flows, err := s.flows.ListEnabledMigratedBySource(ctx, msg.SourceID)
	if err != nil || len(flows) == 0 {
		return nil, err
	}
	return s.flowEngine.Evaluate(ctx, msg, flows)
}

func (s *Service) shadowCompare(ctx context.Context, msg *domainmessage.NormalizedMessage, ruleMatches []ruleengine.Match) {
	flowMatches, err := s.evaluateMigratedFlows(ctx, msg)
	if err != nil {
		s.log.Warn("Flow 影子求值失败", "message_id", msg.ID, "source_id", msg.SourceID, "err", err)
		return
	}
	ruleSig := matchSignature(ruleMatches)
	flowSig := matchSignature(flowMatches)
	if strings.Join(ruleSig, "|") == strings.Join(flowSig, "|") {
		return
	}
	s.log.Warn("Flow 影子求值 diff",
		"message_id", msg.ID,
		"source_id", msg.SourceID,
		"rule_matches", len(ruleMatches),
		"flow_matches", len(flowMatches),
		"rule_signature", ruleSig,
		"flow_signature", flowSig,
		"rule_detail", matchDetailSignature(ruleMatches),
		"flow_detail", matchDetailSignature(flowMatches),
	)
}

func matchSignature(matches []ruleengine.Match) []string {
	out := make([]string, 0)
	for _, m := range matches {
		for _, target := range m.Targets {
			tpl := "0"
			if target.TemplateID != nil {
				tpl = strconv.FormatInt(*target.TemplateID, 10)
			}
			out = append(out, strconv.FormatInt(target.SinkID, 10)+":"+tpl+":"+textDigest(m.Message))
		}
	}
	sort.Strings(out)
	return out
}

func matchDetailSignature(matches []ruleengine.Match) []string {
	out := make([]string, 0)
	for _, m := range matches {
		origin := matchOriginSignature(m)
		for _, target := range m.Targets {
			tpl := "0"
			if target.TemplateID != nil {
				tpl = strconv.FormatInt(*target.TemplateID, 10)
			}
			out = append(out, origin+":sink:"+strconv.FormatInt(target.SinkID, 10)+":tpl:"+tpl+":text:"+textDigest(m.Message))
		}
	}
	sort.Strings(out)
	return out
}

func matchOriginSignature(m ruleengine.Match) string {
	if m.OriginType == "flow" {
		return "flow:" + strconv.FormatInt(m.OriginID, 10) + ":node:" + strconv.FormatInt(m.OriginNodeID, 10)
	}
	ruleID := int64(0)
	if m.Rule != nil {
		ruleID = m.Rule.ID
	}
	return "rule:" + strconv.FormatInt(ruleID, 10)
}

func textDigest(msg *domainmessage.NormalizedMessage) string {
	text := ""
	if msg != nil {
		text = msg.Text
	}
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:8])
}

// persistMedia 把 Source 下载到临时目录的媒体文件移入媒体存储，
// 并把 LocalPath 更新为存储层路径。失败不阻断消息主链路。
func (s *Service) persistMedia(ctx context.Context, msg *domainmessage.NormalizedMessage) {
	if s.media == nil {
		return
	}
	for i := range msg.Media {
		m := &msg.Media[i]
		if m.LocalPath == "" || m.StorageKey == "" {
			continue
		}
		newPath, err := s.media.PutFile(ctx, m.StorageKey, m.LocalPath)
		if newPath != "" {
			// 对象存储上传失败时文件仍已移入本地缓存，保留本地路径降级使用。
			m.LocalPath = newPath
		}
		if err != nil {
			s.log.Warn("媒体收编存储失败",
				"source_id", msg.SourceID,
				"storage_key", m.StorageKey,
				"err", err,
			)
		}
	}
}
