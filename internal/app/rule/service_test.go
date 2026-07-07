package rule

import (
	"context"
	"testing"

	domainfilter "telegram-message-forward/internal/domain/filter"
	domainrule "telegram-message-forward/internal/domain/rule"
)

type fakeFilterRepo struct {
	items map[int64]*domainfilter.Filter
}

func (r fakeFilterRepo) List(context.Context) ([]*domainfilter.Filter, error) { return nil, nil }
func (r fakeFilterRepo) GetByID(_ context.Context, id int64) (*domainfilter.Filter, error) {
	return r.items[id], nil
}
func (r fakeFilterRepo) Create(context.Context, *domainfilter.Filter) error { return nil }
func (r fakeFilterRepo) Update(context.Context, *domainfilter.Filter) error { return nil }
func (r fakeFilterRepo) Delete(context.Context, int64) error                { return nil }
func (r fakeFilterRepo) CountReferences(context.Context, int64) (int64, error) {
	return 0, nil
}

func TestPreviewMergesMultipleFilters(t *testing.T) {
	svc := NewService(nil, ValidatorDeps{Filters: fakeFilterRepo{items: map[int64]*domainfilter.Filter{
		1: {
			ID:         1,
			Conditions: []domainrule.ConditionConfig{{Type: "keyword_contains", Config: map[string]any{"keywords": []any{"golang"}}}},
		},
		2: {
			ID:         2,
			Conditions: []domainrule.ConditionConfig{{Type: "keyword_excludes", Config: map[string]any{"keywords": []any{"广告"}}}},
		},
	}}})

	ctx := context.Background()
	base := PreviewInput{
		Rule: Input{Name: "multi-filter", Enabled: true, FilterIDs: []int64{1, 2}},
		Message: PreviewMessage{
			MessageType: "text",
		},
	}

	base.Message.Text = "golang 每日更新"
	result, err := svc.Preview(ctx, base)
	if err != nil {
		t.Fatalf("preview should pass: %v", err)
	}
	if !result.Matched {
		t.Fatal("同时通过两个过滤器时规则应命中")
	}

	base.Message.Text = "golang 广告"
	result, err = svc.Preview(ctx, base)
	if err != nil {
		t.Fatalf("preview should evaluate filters: %v", err)
	}
	if result.Matched {
		t.Fatal("命中阻止关键词过滤器时规则不应命中")
	}
}
