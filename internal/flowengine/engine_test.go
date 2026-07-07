package flowengine

import (
	"context"
	"sort"
	"strconv"
	"testing"
	"time"

	domainflow "telegram-message-forward/internal/domain/flow"
	domainmessage "telegram-message-forward/internal/domain/message"
)

func TestEvaluateDiamondDedupesTargetNode(t *testing.T) {
	src, f1, f2, target := int64(1), int64(2), int64(3), int64(4)
	sinkID := int64(100)
	flow := &domainflow.Flow{
		ID: 1, Name: "diamond", Enabled: true, UpdatedAt: time.Now(),
		Nodes: []domainflow.Node{
			sourceNode(src, 10),
			filterNode(f1, contains("go")),
			filterNode(f2, contains("go")),
			targetNode(target, sinkID, nil),
		},
		Edges: []domainflow.Edge{
			{FromNodeID: src, ToNodeID: f1},
			{FromNodeID: src, ToNodeID: f2},
			{FromNodeID: f1, ToNodeID: target},
			{FromNodeID: f2, ToNodeID: target},
		},
	}
	matches, err := NewEngine().Evaluate(context.Background(), msg("golang"), []*domainflow.Flow{flow})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len=%d, want 1", len(matches))
	}
	if matches[0].OriginNodeID != target {
		t.Fatalf("OriginNodeID=%d, want %d", matches[0].OriginNodeID, target)
	}
}

func TestEvaluateFanoutClonesBranchSnapshot(t *testing.T) {
	src, proc, target1, target2 := int64(1), int64(2), int64(3), int64(4)
	sink1, sink2 := int64(100), int64(200)
	flow := &domainflow.Flow{
		ID: 1, Name: "fanout", Enabled: true, UpdatedAt: time.Now(),
		Nodes: []domainflow.Node{
			sourceNode(src, 10),
			processorNode(proc, domainflow.ProcessorConfig{Type: "append_source", Config: map[string]any{"text": " [p]"}}),
			targetNode(target1, sink1, nil),
			targetNode(target2, sink2, nil),
		},
		Edges: []domainflow.Edge{
			{FromNodeID: src, ToNodeID: proc},
			{FromNodeID: src, ToNodeID: target2},
			{FromNodeID: proc, ToNodeID: target1},
		},
	}
	matches, err := NewEngine().Evaluate(context.Background(), msg("hello"), []*domainflow.Flow{flow})
	if err != nil {
		t.Fatal(err)
	}
	textBySink := map[int64]string{}
	for _, m := range matches {
		textBySink[m.Targets[0].SinkID] = m.Message.Text
	}
	if textBySink[sink1] != "hello [p]" {
		t.Fatalf("processed branch text=%q", textBySink[sink1])
	}
	if textBySink[sink2] != "hello" {
		t.Fatalf("sibling branch text=%q", textBySink[sink2])
	}
}

func TestEvaluateMultiSourceAndMultiLevelFilters(t *testing.T) {
	sinkID := int64(100)
	flow := &domainflow.Flow{
		ID: 1, Name: "multi-source", Enabled: true, UpdatedAt: time.Now(),
		Nodes: []domainflow.Node{
			sourceNode(1, 10),
			sourceNode(2, 20),
			filterNode(3, contains("go")),
			filterNode(4, domainflow.ConditionConfig{Type: "message_type", Config: map[string]any{"types": []any{"text"}}}),
			targetNode(5, sinkID, nil),
		},
		Edges: []domainflow.Edge{
			{FromNodeID: 1, ToNodeID: 3},
			{FromNodeID: 2, ToNodeID: 3},
			{FromNodeID: 3, ToNodeID: 4},
			{FromNodeID: 4, ToNodeID: 5},
		},
	}
	m := msg("go news")
	m.SourceID = 20
	matches, err := NewEngine().Evaluate(context.Background(), m, []*domainflow.Flow{flow})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches len=%d, want 1", len(matches))
	}
	m.Text = "rust news"
	matches, err = NewEngine().Evaluate(context.Background(), m, []*domainflow.Flow{flow})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("matches len=%d, want 0", len(matches))
	}
}

func TestEvaluateStopOnMatchBetweenFlows(t *testing.T) {
	high := linearFlow(1, 10, 100, contains("go"), nil)
	high.Priority = 20
	high.StopOnMatch = true
	low := linearFlow(2, 10, 200, contains("go"), nil)
	low.Priority = 1
	matches, err := NewEngine().Evaluate(context.Background(), msg("go"), []*domainflow.Flow{low, high})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].OriginID != high.ID {
		t.Fatalf("matches=%v, want only high priority flow", signatures(matches))
	}
}

func TestEvaluateLinearFlowWithProcessorsAndMultipleTargets(t *testing.T) {
	tpl := int64(9)
	conds := []domainflow.ConditionConfig{
		contains("go"),
		{Type: "message_type", Config: map[string]any{"types": []any{"text"}}},
	}
	procs := []domainflow.ProcessorConfig{{Type: "append_source", Config: map[string]any{"text": " [p]"}}}
	flow := &domainflow.Flow{
		ID: 7, Name: "linear", Enabled: true, Priority: 10, StopOnMatch: true, UpdatedAt: time.Now(),
		Nodes: []domainflow.Node{
			sourceNode(1, 10),
			filterNode(2, conds...),
			processorNode(3, procs...),
			targetNode(4, 100, &tpl),
			targetNode(5, 200, nil),
		},
		Edges: []domainflow.Edge{
			{FromNodeID: 1, ToNodeID: 2},
			{FromNodeID: 2, ToNodeID: 3},
			{FromNodeID: 3, ToNodeID: 4},
			{FromNodeID: 3, ToNodeID: 5},
		},
	}
	flowMatches, err := NewEngine().Evaluate(context.Background(), msg("go"), []*domainflow.Flow{flow})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"100:9:go [p]", "200:0:go [p]"}
	if got := signatures(flowMatches); !sameStrings(got, want) {
		t.Fatalf("flow signatures=%v, want %v", got, want)
	}
}

func sourceNode(id, sourceID int64) domainflow.Node {
	return domainflow.Node{ID: id, Type: domainflow.NodeTypeSource, RefID: &sourceID}
}

func filterNode(id int64, conds ...domainflow.ConditionConfig) domainflow.Node {
	return domainflow.Node{ID: id, Type: domainflow.NodeTypeFilter, Config: domainflow.NodeConfig{Conditions: conds}}
}

func processorNode(id int64, procs ...domainflow.ProcessorConfig) domainflow.Node {
	return domainflow.Node{ID: id, Type: domainflow.NodeTypeProcessor, Config: domainflow.NodeConfig{Processors: procs}}
}

func targetNode(id, sinkID int64, templateID *int64) domainflow.Node {
	return domainflow.Node{ID: id, Type: domainflow.NodeTypeTarget, RefID: &sinkID, TemplateID: templateID}
}

func linearFlow(id, sourceID, sinkID int64, cond domainflow.ConditionConfig, tpl *int64) *domainflow.Flow {
	return &domainflow.Flow{
		ID: id, Name: "linear", Enabled: true, UpdatedAt: time.Now(),
		Nodes: []domainflow.Node{sourceNode(1, sourceID), filterNode(2, cond), targetNode(3, sinkID, tpl)},
		Edges: []domainflow.Edge{{FromNodeID: 1, ToNodeID: 2}, {FromNodeID: 2, ToNodeID: 3}},
	}
}

func contains(keyword string) domainflow.ConditionConfig {
	return domainflow.ConditionConfig{Type: "keyword_contains", Config: map[string]any{"keywords": []any{keyword}}}
}

func msg(text string) *domainmessage.NormalizedMessage {
	return &domainmessage.NormalizedMessage{SourceID: 10, MessageType: "text", Text: text, ReceivedAt: time.Now()}
}

func signatures(matches []domainflow.Match) []string {
	out := make([]string, 0)
	for _, m := range matches {
		for _, target := range m.Targets {
			tpl := int64(0)
			if target.TemplateID != nil {
				tpl = *target.TemplateID
			}
			out = append(out, strconv.FormatInt(target.SinkID, 10)+":"+strconv.FormatInt(tpl, 10)+":"+m.Message.Text)
		}
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
