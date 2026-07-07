// Command flowmigrate migrates existing linear rules into executable flows.
package main

import (
	"context"
	"flag"
	"log"

	"gorm.io/gorm"

	appflow "telegram-message-forward/internal/app/flow"
	"telegram-message-forward/internal/config"
	domainflow "telegram-message-forward/internal/domain/flow"
	domainrule "telegram-message-forward/internal/domain/rule"
	"telegram-message-forward/internal/flowengine"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage"
	"telegram-message-forward/internal/storage/repository"
)

func main() {
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	db, err := storage.Open(cfg.Database)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	cipher, err := crypto.NewCipher([]byte(cfg.Security.EncryptionKey))
	if err != nil {
		log.Fatalf("初始化加密失败: %v", err)
	}

	ctx := context.Background()
	rules := repository.NewRuleRepository(db)
	flows := repository.NewFlowRepository(db)
	sources := repository.NewSourceRepository(db)
	sinks := repository.NewSinkRepository(db, cipher)
	templates := repository.NewTemplateRepository(db)
	filters := repository.NewFilterRepository(db)
	engine := flowengine.NewEngine(flowengine.WithFilterResolver(filters))
	svc := appflow.NewService(flows, engine, appflow.ValidatorDeps{
		Sources: sources, Sinks: sinks, Templates: templates, Filters: filters,
	})

	items, err := rules.List(ctx)
	if err != nil {
		log.Fatalf("读取规则失败: %v", err)
	}
	var created, updated int
	for _, rule := range items {
		input := flowInputFromRule(rule)
		flowID, found, err := mappedFlowID(ctx, db, rule.ID)
		if err != nil {
			log.Fatalf("读取规则 %d 的迁移映射失败: %v", rule.ID, err)
		}
		var f *domainflow.Flow
		if found {
			f, err = svc.Update(ctx, flowID, input)
			updated++
		} else {
			f, err = svc.Create(ctx, input)
			created++
		}
		if err != nil {
			log.Fatalf("迁移规则 %d (%s) 失败: %v", rule.ID, rule.Name, err)
		}
		if err := saveMapping(ctx, db, rule.ID, f.ID); err != nil {
			log.Fatalf("保存规则 %d 的迁移映射失败: %v", rule.ID, err)
		}
		log.Printf("规则 %d -> Flow %d: %s", rule.ID, f.ID, f.Name)
	}
	log.Printf("flowmigrate 完成: created=%d updated=%d total=%d", created, updated, len(items))
}

func flowInputFromRule(r *domainrule.Rule) appflow.Input {
	var nodes []domainflow.Node
	var edges []domainflow.Edge
	nextID := int64(-1)
	newNode := func(t domainflow.NodeType, ref *int64, cfg domainflow.NodeConfig, tpl *int64, x, y float64) int64 {
		id := nextID
		nextID--
		nodes = append(nodes, domainflow.Node{ID: id, Type: t, RefID: ref, Config: cfg, TemplateID: tpl, PosX: x, PosY: y})
		return id
	}
	sourceIDs := make([]int64, 0, len(r.SourceIDs))
	for i, sourceID := range r.SourceIDs {
		ref := sourceID
		sourceIDs = append(sourceIDs, newNode(domainflow.NodeTypeSource, &ref, domainflow.NodeConfig{}, nil, 0, float64(i)*120))
	}
	stageIDs := sourceIDs
	if len(r.FilterIDs) > 0 || len(r.Conditions) > 0 {
		cfg := domainflow.NodeConfig{FilterIDs: r.FilterIDs}
		if len(r.FilterIDs) == 0 {
			cfg.Conditions = r.Conditions
		}
		filterID := newNode(domainflow.NodeTypeFilter, nil, cfg, nil, 280, 0)
		for _, from := range stageIDs {
			edges = append(edges, domainflow.Edge{FromNodeID: from, ToNodeID: filterID})
		}
		stageIDs = []int64{filterID}
	}
	if len(r.Processors) > 0 {
		processorID := newNode(domainflow.NodeTypeProcessor, nil, domainflow.NodeConfig{Processors: r.Processors}, nil, 560, 0)
		for _, from := range stageIDs {
			edges = append(edges, domainflow.Edge{FromNodeID: from, ToNodeID: processorID})
		}
		stageIDs = []int64{processorID}
	}
	for i, target := range r.Targets {
		ref := target.SinkID
		targetID := newNode(domainflow.NodeTypeTarget, &ref, domainflow.NodeConfig{}, target.TemplateID, 840, float64(i)*120)
		for _, from := range stageIDs {
			edges = append(edges, domainflow.Edge{FromNodeID: from, ToNodeID: targetID})
		}
	}
	return appflow.Input{
		Name:        r.Name,
		Enabled:     r.Enabled,
		Priority:    r.Priority,
		StopOnMatch: r.StopOnMatch,
		Nodes:       nodes,
		Edges:       edges,
	}
}

func mappedFlowID(ctx context.Context, db *gorm.DB, ruleID int64) (int64, bool, error) {
	var row struct {
		FlowID int64
	}
	err := db.WithContext(ctx).
		Raw("SELECT flow_id FROM flow_rule_migrations WHERE rule_id = ?", ruleID).
		Scan(&row).Error
	if err != nil {
		return 0, false, err
	}
	if row.FlowID <= 0 {
		return 0, false, nil
	}
	return row.FlowID, true, nil
}

func saveMapping(ctx context.Context, db *gorm.DB, ruleID, flowID int64) error {
	return db.WithContext(ctx).Exec(`
INSERT INTO flow_rule_migrations (rule_id, flow_id, updated_at)
VALUES (?, ?, now())
ON CONFLICT (rule_id)
DO UPDATE SET flow_id = EXCLUDED.flow_id, updated_at = now()`, ruleID, flowID).Error
}
