package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	domainflow "telegram-message-forward/internal/domain/flow"
	"telegram-message-forward/internal/storage/model"
)

// FlowRepository 是 flow.Repository 的 PostgreSQL 实现。
type FlowRepository struct {
	db *gorm.DB
}

func NewFlowRepository(db *gorm.DB) *FlowRepository {
	return &FlowRepository{db: db}
}

var _ domainflow.Repository = (*FlowRepository)(nil)

func (r *FlowRepository) Create(ctx context.Context, f *domainflow.Flow) error {
	m := toFlowModel(f)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		f.ID = m.ID
		if err := r.replaceGraph(tx, f); err != nil {
			return err
		}
		loaded, err := r.getByIDTx(tx.WithContext(ctx), f.ID)
		if err != nil {
			return err
		}
		*f = *loaded
		return nil
	})
}

func (r *FlowRepository) Update(ctx context.Context, f *domainflow.Flow) error {
	m := toFlowModel(f)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(m).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", f.ID).Delete(&model.FlowEdge{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", f.ID).Delete(&model.FlowNode{}).Error; err != nil {
			return err
		}
		if err := r.replaceGraph(tx, f); err != nil {
			return err
		}
		loaded, err := r.getByIDTx(tx.WithContext(ctx), f.ID)
		if err != nil {
			return err
		}
		*f = *loaded
		return nil
	})
}

func (r *FlowRepository) GetByID(ctx context.Context, id int64) (*domainflow.Flow, error) {
	return r.getByIDTx(r.db.WithContext(ctx), id)
}

func (r *FlowRepository) List(ctx context.Context) ([]*domainflow.Flow, error) {
	var ms []model.Flow
	if err := r.db.WithContext(ctx).Order("priority DESC, id").Find(&ms).Error; err != nil {
		return nil, err
	}
	return r.assemble(ctx, ms)
}

func (r *FlowRepository) ListEnabledBySource(ctx context.Context, sourceID int64) ([]*domainflow.Flow, error) {
	var ms []model.Flow
	err := r.db.WithContext(ctx).
		Joins("JOIN flow_nodes fn ON fn.flow_id = flows.id").
		Where("fn.type = ? AND fn.ref_id = ? AND flows.enabled = ?", string(domainflow.NodeTypeSource), sourceID, true).
		Group("flows.id").
		Order("flows.priority DESC, flows.id").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return r.assemble(ctx, ms)
}

func (r *FlowRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.Flow{}, id).Error
}

func (r *FlowRepository) replaceGraph(tx *gorm.DB, f *domainflow.Flow) error {
	oldToNew := map[int64]int64{}
	for i := range f.Nodes {
		n := f.Nodes[i]
		n.FlowID = f.ID
		m, err := toFlowNodeModel(n)
		if err != nil {
			return err
		}
		oldID := n.ID
		m.ID = 0
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		f.Nodes[i].ID = m.ID
		f.Nodes[i].FlowID = f.ID
		if oldID != 0 {
			oldToNew[oldID] = m.ID
		}
	}
	for i := range f.Edges {
		e := f.Edges[i]
		e.FlowID = f.ID
		fromID := remapNodeID(e.FromNodeID, oldToNew)
		toID := remapNodeID(e.ToNodeID, oldToNew)
		if fromID <= 0 || toID <= 0 {
			return fmt.Errorf("flow edge 引用不存在的节点: %d -> %d", e.FromNodeID, e.ToNodeID)
		}
		m := &model.FlowEdge{FlowID: f.ID, FromNodeID: fromID, ToNodeID: toID}
		if err := tx.Create(m).Error; err != nil {
			return err
		}
		f.Edges[i].ID = m.ID
		f.Edges[i].FlowID = f.ID
		f.Edges[i].FromNodeID = fromID
		f.Edges[i].ToNodeID = toID
	}
	return nil
}

func remapNodeID(id int64, mapping map[int64]int64) int64 {
	if id == 0 {
		return 0
	}
	if mapped, ok := mapping[id]; ok {
		return mapped
	}
	return id
}

func (r *FlowRepository) assemble(ctx context.Context, ms []model.Flow) ([]*domainflow.Flow, error) {
	out := make([]*domainflow.Flow, 0, len(ms))
	for i := range ms {
		f, err := r.loadFull(ctx, &ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func (r *FlowRepository) getByIDTx(db *gorm.DB, id int64) (*domainflow.Flow, error) {
	var m model.Flow
	if err := db.First(&m, id).Error; err != nil {
		return nil, err
	}
	return r.loadFullDB(db, &m)
}

func (r *FlowRepository) loadFull(ctx context.Context, m *model.Flow) (*domainflow.Flow, error) {
	return r.loadFullDB(r.db.WithContext(ctx), m)
}

func (r *FlowRepository) loadFullDB(db *gorm.DB, m *model.Flow) (*domainflow.Flow, error) {
	f := toFlowDomain(m)
	var ns []model.FlowNode
	if err := db.Where("flow_id = ?", m.ID).Order("id").Find(&ns).Error; err != nil {
		return nil, err
	}
	f.Nodes = make([]domainflow.Node, 0, len(ns))
	for i := range ns {
		n, err := toFlowNodeDomain(&ns[i])
		if err != nil {
			return nil, err
		}
		f.Nodes = append(f.Nodes, n)
	}
	var es []model.FlowEdge
	if err := db.Where("flow_id = ?", m.ID).Order("id").Find(&es).Error; err != nil {
		return nil, err
	}
	f.Edges = make([]domainflow.Edge, 0, len(es))
	for _, e := range es {
		f.Edges = append(f.Edges, domainflow.Edge{
			ID: e.ID, FlowID: e.FlowID, FromNodeID: e.FromNodeID, ToNodeID: e.ToNodeID,
		})
	}
	return f, nil
}

func toFlowModel(f *domainflow.Flow) *model.Flow {
	return &model.Flow{
		ID:          f.ID,
		Name:        f.Name,
		Enabled:     f.Enabled,
		Priority:    f.Priority,
		StopOnMatch: f.StopOnMatch,
		CreatedAt:   f.CreatedAt,
		UpdatedAt:   f.UpdatedAt,
	}
}

func toFlowDomain(m *model.Flow) *domainflow.Flow {
	return &domainflow.Flow{
		ID:          m.ID,
		Name:        m.Name,
		Enabled:     m.Enabled,
		Priority:    m.Priority,
		StopOnMatch: m.StopOnMatch,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
}

func toFlowNodeModel(n domainflow.Node) (*model.FlowNode, error) {
	cfg, err := marshalJSON(n.Config)
	if err != nil {
		return nil, fmt.Errorf("序列化 flow node config 失败: %w", err)
	}
	return &model.FlowNode{
		ID: n.ID, FlowID: n.FlowID, Type: string(n.Type), RefID: n.RefID,
		Config: cfg, TemplateID: n.TemplateID, PosX: n.PosX, PosY: n.PosY,
	}, nil
}

func toFlowNodeDomain(m *model.FlowNode) (domainflow.Node, error) {
	var cfg domainflow.NodeConfig
	if err := unmarshalJSON(m.Config, &cfg); err != nil {
		return domainflow.Node{}, fmt.Errorf("解析 flow node config 失败: %w", err)
	}
	return domainflow.Node{
		ID: m.ID, FlowID: m.FlowID, Type: domainflow.NodeType(m.Type), RefID: m.RefID,
		Config: cfg, TemplateID: m.TemplateID, PosX: m.PosX, PosY: m.PosY,
	}, nil
}
