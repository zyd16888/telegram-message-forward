// Package flowengine 编译并执行 Flow 图编排。
package flowengine

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	domainfilter "telegram-message-forward/internal/domain/filter"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/ruleengine"
	"telegram-message-forward/internal/ruleengine/condition"
	"telegram-message-forward/internal/ruleengine/processor"
)

const (
	maxNodes = 64
	maxDepth = 16
)

type FilterResolver interface {
	GetByID(ctx context.Context, id int64) (*domainfilter.Filter, error)
}

type Engine struct {
	filters FilterResolver
	mu      sync.Mutex
	cache   map[int64]cachedPlan
}

type Option func(*Engine)

func WithFilterResolver(filters FilterResolver) Option {
	return func(e *Engine) { e.filters = filters }
}

func NewEngine(opts ...Option) *Engine {
	e := &Engine{cache: map[int64]cachedPlan{}}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type cachedPlan struct {
	updatedAt time.Time
	plan      *Plan
}

type Plan struct {
	flow      *domainflow.Flow
	nodes     map[int64]domainflow.Node
	out       map[int64][]int64
	sourceIDs map[int64][]int64
}

func (e *Engine) Evaluate(ctx context.Context, msg *domainmessage.NormalizedMessage, flows []*domainflow.Flow) ([]ruleengine.Match, error) {
	if msg == nil {
		return nil, nil
	}
	ordered := append([]*domainflow.Flow(nil), flows...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].ID < ordered[j].ID
		}
		return ordered[i].Priority > ordered[j].Priority
	})

	var matches []ruleengine.Match
	for _, f := range ordered {
		if f == nil || !f.Enabled {
			continue
		}
		plan, err := e.compileCached(f)
		if err != nil {
			return nil, err
		}
		got, err := e.evaluatePlan(ctx, msg, plan)
		if err != nil {
			return nil, err
		}
		if len(got) == 0 {
			continue
		}
		matches = append(matches, got...)
		if f.StopOnMatch {
			break
		}
	}
	return matches, nil
}

func (e *Engine) Compile(f *domainflow.Flow) (*Plan, error) {
	if f == nil {
		return nil, fmt.Errorf("flow 不能为空")
	}
	if len(f.Nodes) == 0 {
		return nil, fmt.Errorf("flow 至少需要一个节点")
	}
	if len(f.Nodes) > maxNodes {
		return nil, fmt.Errorf("flow 节点数不能超过 %d", maxNodes)
	}

	p := &Plan{
		flow:      f,
		nodes:     map[int64]domainflow.Node{},
		out:       map[int64][]int64{},
		sourceIDs: map[int64][]int64{},
	}
	inDegree := map[int64]int{}
	outDegree := map[int64]int{}
	for _, n := range f.Nodes {
		if n.ID == 0 {
			return nil, fmt.Errorf("节点 id 不能为空")
		}
		if _, ok := p.nodes[n.ID]; ok {
			return nil, fmt.Errorf("节点 id 重复: %d", n.ID)
		}
		if err := validateNode(n); err != nil {
			return nil, err
		}
		p.nodes[n.ID] = n
		inDegree[n.ID] = 0
		outDegree[n.ID] = 0
		if n.Type == domainflow.NodeTypeSource && n.RefID != nil {
			p.sourceIDs[*n.RefID] = append(p.sourceIDs[*n.RefID], n.ID)
		}
	}
	for _, edge := range f.Edges {
		if edge.FromNodeID == edge.ToNodeID {
			return nil, fmt.Errorf("连线不能指向自身: node=%d", edge.FromNodeID)
		}
		if _, ok := p.nodes[edge.FromNodeID]; !ok {
			return nil, fmt.Errorf("连线起点不存在: node=%d", edge.FromNodeID)
		}
		if _, ok := p.nodes[edge.ToNodeID]; !ok {
			return nil, fmt.Errorf("连线终点不存在: node=%d", edge.ToNodeID)
		}
		p.out[edge.FromNodeID] = append(p.out[edge.FromNodeID], edge.ToNodeID)
		inDegree[edge.ToNodeID]++
		outDegree[edge.FromNodeID]++
	}
	for _, n := range f.Nodes {
		switch n.Type {
		case domainflow.NodeTypeSource:
			if inDegree[n.ID] != 0 {
				return nil, fmt.Errorf("source 节点只能作为入口: node=%d", n.ID)
			}
			if outDegree[n.ID] == 0 {
				return nil, fmt.Errorf("source 节点至少需要一条出边: node=%d", n.ID)
			}
		case domainflow.NodeTypeTarget:
			if outDegree[n.ID] != 0 {
				return nil, fmt.Errorf("target 节点只能作为出口: node=%d", n.ID)
			}
			if inDegree[n.ID] == 0 {
				return nil, fmt.Errorf("target 节点至少需要一条入边: node=%d", n.ID)
			}
		}
	}
	if err := validateDAG(p.nodes, p.out, inDegree); err != nil {
		return nil, err
	}
	if !hasSourceToTargetPath(p) {
		return nil, fmt.Errorf("flow 至少需要一条 source 到 target 的通路")
	}
	return p, nil
}

func (e *Engine) compileCached(f *domainflow.Flow) (*Plan, error) {
	e.mu.Lock()
	if cached, ok := e.cache[f.ID]; ok && cached.updatedAt.Equal(f.UpdatedAt) {
		e.mu.Unlock()
		return cached.plan, nil
	}
	e.mu.Unlock()

	plan, err := e.Compile(f)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.cache[f.ID] = cachedPlan{updatedAt: f.UpdatedAt, plan: plan}
	e.mu.Unlock()
	return plan, nil
}

func validateNode(n domainflow.Node) error {
	switch n.Type {
	case domainflow.NodeTypeSource:
		if n.RefID == nil || *n.RefID <= 0 {
			return fmt.Errorf("source 节点缺少有效 ref_id: node=%d", n.ID)
		}
	case domainflow.NodeTypeTarget:
		if n.RefID == nil || *n.RefID <= 0 {
			return fmt.Errorf("target 节点缺少有效 ref_id: node=%d", n.ID)
		}
	case domainflow.NodeTypeFilter:
		if len(n.Config.FilterIDs) > 0 && len(n.Config.Conditions) > 0 {
			return fmt.Errorf("filter 节点 filter_ids 与 conditions 只能二选一: node=%d", n.ID)
		}
		if len(n.Config.FilterIDs) == 0 && len(n.Config.Conditions) == 0 {
			return fmt.Errorf("filter 节点至少需要 filter_ids 或 conditions: node=%d", n.ID)
		}
		for _, cfg := range n.Config.Conditions {
			if err := condition.ValidateConfig(cfg.Type, cfg.Config); err != nil {
				return fmt.Errorf("filter 节点条件 %s 配置无效: %w", cfg.Type, err)
			}
		}
	case domainflow.NodeTypeProcessor:
		if len(n.Config.Processors) == 0 {
			return fmt.Errorf("processor 节点至少需要一个处理器: node=%d", n.ID)
		}
		for _, cfg := range n.Config.Processors {
			if err := processor.ValidateConfig(cfg.Type, cfg.Config); err != nil {
				return fmt.Errorf("processor 节点处理器 %s 配置无效: %w", cfg.Type, err)
			}
		}
	default:
		return fmt.Errorf("未知节点类型 %q: node=%d", n.Type, n.ID)
	}
	return nil
}

func validateDAG(nodes map[int64]domainflow.Node, out map[int64][]int64, inDegree map[int64]int) error {
	deg := map[int64]int{}
	queue := make([]int64, 0)
	depth := map[int64]int{}
	for id := range nodes {
		deg[id] = inDegree[id]
		if deg[id] == 0 {
			queue = append(queue, id)
		}
	}
	sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
	visited := 0
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		visited++
		if depth[id] > maxDepth {
			return fmt.Errorf("flow 深度不能超过 %d", maxDepth)
		}
		for _, to := range out[id] {
			if depth[id]+1 > depth[to] {
				depth[to] = depth[id] + 1
			}
			deg[to]--
			if deg[to] == 0 {
				queue = append(queue, to)
				sort.Slice(queue, func(i, j int) bool { return queue[i] < queue[j] })
			}
		}
	}
	if visited != len(nodes) {
		return fmt.Errorf("flow 图必须是 DAG，不能包含环")
	}
	return nil
}

func hasSourceToTargetPath(p *Plan) bool {
	for _, sourceNodes := range p.sourceIDs {
		for _, sourceID := range sourceNodes {
			if reachesTarget(p, sourceID, map[int64]bool{}) {
				return true
			}
		}
	}
	return false
}

func reachesTarget(p *Plan, id int64, seen map[int64]bool) bool {
	if seen[id] {
		return false
	}
	seen[id] = true
	if p.nodes[id].Type == domainflow.NodeTypeTarget {
		return true
	}
	for _, to := range p.out[id] {
		if reachesTarget(p, to, seen) {
			return true
		}
	}
	return false
}

type evalState struct {
	nodeID int64
	msg    *domainmessage.NormalizedMessage
}

func (e *Engine) evaluatePlan(ctx context.Context, msg *domainmessage.NormalizedMessage, p *Plan) ([]ruleengine.Match, error) {
	starts := p.sourceIDs[msg.SourceID]
	if len(starts) == 0 {
		return nil, nil
	}
	queue := make([]evalState, 0, len(starts))
	for _, id := range starts {
		queue = append(queue, evalState{nodeID: id, msg: cloneMessage(msg)})
	}
	seenTargets := map[int64]bool{}
	var matches []ruleengine.Match
	for len(queue) > 0 {
		state := queue[0]
		queue = queue[1:]
		node := p.nodes[state.nodeID]
		current, produced, err := e.evalNode(ctx, p.flow, node, state.msg, seenTargets)
		if err != nil {
			return nil, err
		}
		if produced != nil {
			matches = append(matches, *produced)
			continue
		}
		if current == nil {
			continue
		}
		next := p.out[node.ID]
		for _, to := range next {
			queue = append(queue, evalState{nodeID: to, msg: cloneMessage(current)})
		}
	}
	return matches, nil
}

func (e *Engine) evalNode(ctx context.Context, f *domainflow.Flow, n domainflow.Node, msg *domainmessage.NormalizedMessage, seenTargets map[int64]bool) (*domainmessage.NormalizedMessage, *ruleengine.Match, error) {
	switch n.Type {
	case domainflow.NodeTypeSource:
		return msg, nil, nil
	case domainflow.NodeTypeFilter:
		ok, err := e.evalFilter(ctx, msg, n)
		if err != nil || !ok {
			return nil, nil, err
		}
		return msg, nil, nil
	case domainflow.NodeTypeProcessor:
		if err := runProcessors(ctx, msg, n.Config.Processors); err != nil {
			return nil, nil, err
		}
		return msg, nil, nil
	case domainflow.NodeTypeTarget:
		if seenTargets[n.ID] {
			return nil, nil, nil
		}
		seenTargets[n.ID] = true
		if n.RefID == nil {
			return nil, nil, fmt.Errorf("target 节点缺少 ref_id: node=%d", n.ID)
		}
		match := ruleengine.Match{
			Rule: &domainrule.Rule{
				ID:          f.ID,
				Name:        f.Name,
				Enabled:     f.Enabled,
				Priority:    f.Priority,
				StopOnMatch: f.StopOnMatch,
			},
			Targets:      []domainrule.Target{{SinkID: *n.RefID, TemplateID: n.TemplateID}},
			Message:      cloneMessage(msg),
			OriginType:   "flow",
			OriginID:     f.ID,
			OriginNodeID: n.ID,
		}
		return nil, &match, nil
	default:
		return nil, nil, fmt.Errorf("未知节点类型 %q: node=%d", n.Type, n.ID)
	}
}

func (e *Engine) evalFilter(ctx context.Context, msg *domainmessage.NormalizedMessage, n domainflow.Node) (bool, error) {
	conds := append([]domainrule.ConditionConfig(nil), n.Config.Conditions...)
	for _, filterID := range n.Config.FilterIDs {
		if e.filters == nil {
			return false, fmt.Errorf("filter 节点引用共享过滤器但未配置 resolver: node=%d", n.ID)
		}
		f, err := e.filters.GetByID(ctx, filterID)
		if err != nil {
			return false, fmt.Errorf("加载共享过滤器失败 id=%d: %w", filterID, err)
		}
		conds = append(conds, f.Conditions...)
	}
	for _, cfg := range conds {
		c, err := condition.Get(cfg.Type)
		if err != nil {
			return false, err
		}
		ok, err := c.Evaluate(ctx, msg, cfg.Config)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func runProcessors(ctx context.Context, msg *domainmessage.NormalizedMessage, configs []domainrule.ProcessorConfig) error {
	for _, cfg := range configs {
		p, err := processor.Get(cfg.Type)
		if err != nil {
			return err
		}
		if err := p.Process(ctx, msg, cfg.Config); err != nil {
			return err
		}
	}
	return nil
}

func cloneMessage(msg *domainmessage.NormalizedMessage) *domainmessage.NormalizedMessage {
	if msg == nil {
		return nil
	}
	cp := *msg
	if msg.GroupedID != nil {
		v := *msg.GroupedID
		cp.GroupedID = &v
	}
	if msg.Media != nil {
		cp.Media = append([]domainmessage.Media(nil), msg.Media...)
	}
	if msg.Links != nil {
		cp.Links = append([]domainmessage.Link(nil), msg.Links...)
	}
	if msg.RawPayload != nil {
		cp.RawPayload = append([]byte(nil), msg.RawPayload...)
	}
	if msg.SentAt != nil {
		v := *msg.SentAt
		cp.SentAt = &v
	}
	return &cp
}
