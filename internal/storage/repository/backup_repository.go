package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	domainbackup "telegram-message-forward/internal/domain/backup"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"
)

const installationIDKey = "system.installation_id"

type BackupRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

func NewBackupRepository(db *gorm.DB, cipher *crypto.Cipher) *BackupRepository {
	return &BackupRepository{db: db, cipher: cipher}
}

var _ domainbackup.Repository = (*BackupRepository)(nil)

type backupAccount struct {
	model.Account
	AppHash string `json:"app_hash,omitempty"`
	Session []byte `json:"session,omitempty"`
}
type backupTelegramApp struct {
	model.TelegramApp
	AppHash string `json:"app_hash,omitempty"`
}
type backupProxy struct {
	model.ProxyConfig
	Password string `json:"password,omitempty"`
}
type backupSink struct {
	model.Sink
	Secret string `json:"secret,omitempty"`
}
type backupSetting struct {
	model.Setting
	Secret string `json:"secret,omitempty"`
}

type backupPayload struct {
	Manifest          domainbackup.Manifest          `json:"manifest"`
	TelegramApps      []backupTelegramApp            `json:"telegram_apps"`
	Proxies           []backupProxy                  `json:"proxies"`
	Accounts          []backupAccount                `json:"accounts"`
	Sources           []model.Source                 `json:"sources"`
	Sinks             []backupSink                   `json:"sinks"`
	Templates         []model.Template               `json:"templates"`
	Filters           []model.Filter                 `json:"filters"`
	Flows             []model.Flow                   `json:"flows"`
	FlowNodes         []model.FlowNode               `json:"flow_nodes"`
	FlowEdges         []model.FlowEdge               `json:"flow_edges"`
	Settings          []backupSetting                `json:"settings"`
	AIProfiles        []model.AIDigestProfile        `json:"ai_profiles"`
	AIOutputTemplates []model.AIDigestOutputTemplate `json:"ai_output_templates"`
}

func (r *BackupRepository) ExportPayload(ctx context.Context, includeSessions bool) ([]byte, domainbackup.Manifest, error) {
	installationID, err := r.ensureInstallationID(ctx, r.db)
	if err != nil {
		return nil, domainbackup.Manifest{}, err
	}
	p := backupPayload{}
	if err := r.loadPayload(ctx, &p, includeSessions); err != nil {
		return nil, domainbackup.Manifest{}, err
	}
	p.Manifest = domainbackup.Manifest{Version: domainbackup.FormatVersion, InstallationID: installationID, CreatedAt: time.Now().UTC(), IncludesSession: includeSessions}
	p.Manifest.Counts = payloadCounts(&p)
	data, err := json.Marshal(p)
	return data, p.Manifest, err
}

func (r *BackupRepository) InspectPayload(ctx context.Context, payload []byte) (*domainbackup.Preview, error) {
	p, err := decodeBackupPayload(payload)
	if err != nil {
		return nil, err
	}
	targetID, err := r.ensureInstallationID(ctx, r.db)
	if err != nil {
		return nil, err
	}
	empty, err := r.targetEmpty(ctx, r.db)
	if err != nil {
		return nil, err
	}
	same := targetID == p.Manifest.InstallationID
	preview := &domainbackup.Preview{Manifest: p.Manifest, TargetEmpty: empty, SameInstallation: same, CanRestore: empty || same}
	if !preview.CanRestore {
		preview.Warning = "目标实例已有配置且来源实例不同，为避免引用错配已拒绝恢复"
	}
	if same && !empty {
		preview.Warning = "将更新或补回备份中的配置，备份之外的现有对象不会删除"
	}
	return preview, nil
}

func (r *BackupRepository) RestorePayload(ctx context.Context, payload []byte) (*domainbackup.RestoreResult, error) {
	p, err := decodeBackupPayload(payload)
	if err != nil {
		return nil, err
	}
	preview, err := r.InspectPayload(ctx, payload)
	if err != nil {
		return nil, err
	}
	if !preview.CanRestore {
		return nil, errors.New(preview.Warning)
	}
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return r.restore(tx, p, preview.SameInstallation) })
	if err != nil {
		return nil, err
	}
	return &domainbackup.RestoreResult{Counts: p.Manifest.Counts, RestartRequired: true}, nil
}

func decodeBackupPayload(data []byte) (*backupPayload, error) {
	var p backupPayload
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("解析备份内容失败: %w", err)
	}
	if p.Manifest.Version != domainbackup.FormatVersion || p.Manifest.InstallationID == "" {
		return nil, errors.New("不支持的备份内容版本")
	}
	return &p, nil
}

func (r *BackupRepository) loadPayload(ctx context.Context, p *backupPayload, includeSessions bool) error {
	db := r.db.WithContext(ctx)
	var apps []model.TelegramApp
	var proxies []model.ProxyConfig
	var accounts []model.Account
	var sinks []model.Sink
	var settings []model.Setting
	queries := []any{&apps, &proxies, &accounts, &p.Sources, &sinks, &p.Templates, &p.Filters, &p.Flows, &p.FlowNodes, &p.FlowEdges, &p.AIProfiles, &p.AIOutputTemplates}
	for _, out := range queries {
		if err := db.Order("id ASC").Find(out).Error; err != nil {
			return err
		}
	}
	if err := db.Order("key ASC").Find(&settings).Error; err != nil {
		return err
	}
	for _, item := range apps {
		plain, err := r.decrypt(item.AppHashEncrypted)
		if err != nil {
			return err
		}
		item.AppHashEncrypted = nil
		p.TelegramApps = append(p.TelegramApps, backupTelegramApp{TelegramApp: item, AppHash: plain})
	}
	for _, item := range proxies {
		plain, err := r.decrypt(item.PasswordEncrypted)
		if err != nil {
			return err
		}
		item.PasswordEncrypted = nil
		p.Proxies = append(p.Proxies, backupProxy{ProxyConfig: item, Password: plain})
	}
	for _, item := range accounts {
		hash, err := r.decrypt(item.AppHashEncrypted)
		if err != nil {
			return err
		}
		var session []byte
		if includeSessions {
			session, err = r.cipher.Decrypt(item.SessionEncrypted)
			if err != nil {
				return err
			}
		}
		item.AppHashEncrypted, item.SessionEncrypted = nil, nil
		p.Accounts = append(p.Accounts, backupAccount{Account: item, AppHash: hash, Session: session})
	}
	for _, item := range sinks {
		backupItem, err := r.backupSink(item)
		if err != nil {
			return err
		}
		p.Sinks = append(p.Sinks, backupItem)
	}
	for _, item := range settings {
		plain, err := r.decrypt(item.SecretEncrypted)
		if err != nil {
			return err
		}
		item.SecretEncrypted = nil
		p.Settings = append(p.Settings, backupSetting{Setting: item, Secret: plain})
	}
	return nil
}

func (r *BackupRepository) decrypt(value []byte) (string, error) {
	plain, err := r.cipher.Decrypt(value)
	return string(plain), err
}

func (r *BackupRepository) backupSink(item model.Sink) (backupSink, error) {
	secret, err := r.decrypt(item.SecretEncrypted)
	if err != nil {
		return backupSink{}, err
	}
	config := []byte(item.Config)
	if len(item.ConfigEncrypted) > 0 {
		config, err = r.cipher.Decrypt(item.ConfigEncrypted)
		if err != nil {
			return backupSink{}, fmt.Errorf("解密备份渠道配置失败: %w", err)
		}
	}
	if len(config) == 0 {
		config = []byte(`{}`)
	}
	item.Config = datatypes.JSON(config)
	item.ConfigEncrypted = nil
	item.SecretEncrypted = nil
	return backupSink{Sink: item, Secret: secret}, nil
}

func (r *BackupRepository) restoreSink(item backupSink) (model.Sink, error) {
	row := item.Sink
	config := []byte(row.Config)
	if len(config) == 0 {
		config = []byte(`{}`)
	}
	configEncrypted, err := r.cipher.Encrypt(config)
	if err != nil {
		return model.Sink{}, err
	}
	secretEncrypted, err := r.cipher.Encrypt([]byte(item.Secret))
	if err != nil {
		return model.Sink{}, err
	}
	row.Config = datatypes.JSON([]byte(`{}`))
	row.ConfigEncrypted = configEncrypted
	row.SecretEncrypted = secretEncrypted
	return row, nil
}

func payloadCounts(p *backupPayload) domainbackup.Counts {
	return domainbackup.Counts{
		"telegram_apps": len(p.TelegramApps), "proxies": len(p.Proxies), "accounts": len(p.Accounts), "sources": len(p.Sources),
		"sinks": len(p.Sinks), "templates": len(p.Templates), "filters": len(p.Filters), "flows": len(p.Flows),
		"settings": len(p.Settings), "ai_profiles": len(p.AIProfiles), "ai_output_templates": len(p.AIOutputTemplates),
	}
}

func (r *BackupRepository) ensureInstallationID(ctx context.Context, db *gorm.DB) (string, error) {
	var row model.Setting
	err := db.WithContext(ctx).Where("key = ?", installationIDKey).First(&row).Error
	if err == nil {
		var value struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(row.Value, &value) == nil && value.ID != "" {
			return value.ID, nil
		}
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}
	id := uuid.NewString()
	value, _ := json.Marshal(map[string]string{"id": id})
	row = model.Setting{Key: installationIDKey, Value: datatypes.JSON(value), UpdatedAt: time.Now()}
	result := db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoNothing: true}).Create(&row)
	if result.Error != nil {
		return "", result.Error
	}
	if result.RowsAffected == 0 {
		return r.ensureInstallationID(ctx, db)
	}
	return id, nil
}

func (r *BackupRepository) targetEmpty(ctx context.Context, db *gorm.DB) (bool, error) {
	models := []any{&model.TelegramApp{}, &model.ProxyConfig{}, &model.Account{}, &model.Source{}, &model.Sink{}, &model.Template{}, &model.Filter{}, &model.Flow{}, &model.AIDigestProfile{}}
	for _, item := range models {
		var count int64
		if err := db.WithContext(ctx).Model(item).Count(&count).Error; err != nil {
			return false, err
		}
		if count > 0 {
			return false, nil
		}
	}
	return true, nil
}

func (r *BackupRepository) restore(tx *gorm.DB, p *backupPayload, same bool) error {
	appMap, proxyMap, accountMap := map[int64]int64{}, map[int64]int64{}, map[int64]int64{}
	sourceMap, sinkMap, templateMap, filterMap := map[int64]int64{}, map[int64]int64{}, map[int64]int64{}, map[int64]int64{}
	flowMap, nodeMap, aiTemplateMap := map[int64]int64{}, map[int64]int64{}, map[int64]int64{}

	for _, item := range p.TelegramApps {
		enc, err := r.cipher.Encrypt([]byte(item.AppHash))
		if err != nil {
			return err
		}
		row := item.TelegramApp
		row.AppHashEncrypted = enc
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		appMap[item.ID] = id
	}
	for _, item := range p.Proxies {
		enc, err := r.cipher.Encrypt([]byte(item.Password))
		if err != nil {
			return err
		}
		row := item.ProxyConfig
		row.PasswordEncrypted = enc
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		proxyMap[item.ID] = id
	}
	for _, item := range p.Accounts {
		row := item.Account
		row.TelegramAppID = mappedPtr(row.TelegramAppID, appMap)
		row.ProxyID = mappedPtr(row.ProxyID, proxyMap)
		var err error
		row.AppHashEncrypted, err = r.cipher.Encrypt([]byte(item.AppHash))
		if err != nil {
			return err
		}
		hasSession := len(item.Session) > 0
		if hasSession {
			row.SessionEncrypted, err = r.cipher.Encrypt(item.Session)
			if err != nil {
				return err
			}
		} else if same {
			var current model.Account
			if tx.First(&current, item.ID).Error == nil {
				row.SessionEncrypted = current.SessionEncrypted
				hasSession = len(current.SessionEncrypted) > 0
			}
		}
		if !hasSession {
			row.Status = "inactive"
			row.LastLoginAt = nil
			row.LastError = ""
		}
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		accountMap[item.ID] = id
	}
	for _, item := range p.Sources {
		row := item
		row.AccountID = mappedPtr(row.AccountID, accountMap)
		if same {
			var current model.Source
			if tx.First(&current, item.ID).Error == nil {
				row.LastMessageID = current.LastMessageID
				row.LastSyncedAt = current.LastSyncedAt
			}
		} else {
			row.LastMessageID = 0
			row.LastSyncedAt = nil
		}
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		sourceMap[item.ID] = id
	}
	for _, item := range p.Sinks {
		row, err := r.restoreSink(item)
		if err != nil {
			return err
		}
		if same {
			var current model.Sink
			if tx.First(&current, item.ID).Error == nil {
				row.LastTestAt, row.LastTestSuccess, row.LastTestError = current.LastTestAt, current.LastTestSuccess, current.LastTestError
			}
		} else {
			row.LastTestAt, row.LastTestSuccess, row.LastTestError = nil, false, ""
		}
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		sinkMap[item.ID] = id
	}
	for _, item := range p.Templates {
		row := item
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		templateMap[item.ID] = id
	}
	for _, item := range p.Filters {
		row := item
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		filterMap[item.ID] = id
	}
	for _, item := range p.Flows {
		row := item
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		flowMap[item.ID] = id
		if same {
			if err := tx.Where("flow_id = ?", id).Delete(&model.FlowEdge{}).Error; err != nil {
				return err
			}
			if err := tx.Where("flow_id = ?", id).Delete(&model.FlowNode{}).Error; err != nil {
				return err
			}
		}
	}
	for _, item := range p.FlowNodes {
		row := item
		row.ID = 0
		row.FlowID = flowMap[item.FlowID]
		row.TemplateID = mappedPtr(row.TemplateID, templateMap)
		if row.RefID != nil {
			switch row.Type {
			case "source":
				row.RefID = mappedPtr(row.RefID, sourceMap)
			case "filter":
				row.RefID = mappedPtr(row.RefID, filterMap)
			case "target":
				row.RefID = mappedPtr(row.RefID, sinkMap)
			}
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		nodeMap[item.ID] = row.ID
	}
	for _, item := range p.FlowEdges {
		row := item
		row.ID = 0
		row.FlowID = flowMap[item.FlowID]
		row.FromNodeID = nodeMap[item.FromNodeID]
		row.ToNodeID = nodeMap[item.ToNodeID]
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	for _, item := range p.AIOutputTemplates {
		row := item
		id, err := saveConfigRow(tx, &row, item.ID, same)
		if err != nil {
			return err
		}
		aiTemplateMap[item.ID] = id
	}
	for _, item := range p.AIProfiles {
		row := item
		row.SourceIDs = remapJSONIDs(row.SourceIDs, sourceMap)
		row.TargetSinkIDs = remapJSONIDs(row.TargetSinkIDs, sinkMap)
		row.FilterID = mappedPtr(row.FilterID, filterMap)
		row.OutputTemplateID = mappedPtr(row.OutputTemplateID, aiTemplateMap)
		if _, err := saveConfigRow(tx, &row, item.ID, same); err != nil {
			return err
		}
	}
	for _, item := range p.Settings {
		if item.Key == installationIDKey && !same {
			continue
		}
		row := item.Setting
		enc, err := r.cipher.Encrypt([]byte(item.Secret))
		if err != nil {
			return err
		}
		row.SecretEncrypted = enc
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, UpdateAll: true}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func saveConfigRow[T any](tx *gorm.DB, row *T, backupID int64, same bool) (int64, error) {
	if same && backupID > 0 {
		var count int64
		if err := tx.Model(row).Where("id = ?", backupID).Count(&count).Error; err != nil {
			return 0, err
		}
		if count > 0 {
			if err := tx.Model(row).Where("id = ?", backupID).Select("*").Omit("id").Updates(row).Error; err != nil {
				return 0, err
			}
			return backupID, nil
		}
	}
	setModelID(row, 0)
	if err := tx.Create(row).Error; err != nil {
		return 0, err
	}
	return modelID(row), nil
}

func setModelID(row any, id int64) {
	switch value := row.(type) {
	case *model.TelegramApp:
		value.ID = id
	case *model.ProxyConfig:
		value.ID = id
	case *model.Account:
		value.ID = id
	case *model.Source:
		value.ID = id
	case *model.Sink:
		value.ID = id
	case *model.Template:
		value.ID = id
	case *model.Filter:
		value.ID = id
	case *model.Flow:
		value.ID = id
	case *model.AIDigestProfile:
		value.ID = id
	case *model.AIDigestOutputTemplate:
		value.ID = id
	}
}
func modelID(row any) int64 {
	switch value := row.(type) {
	case *model.TelegramApp:
		return value.ID
	case *model.ProxyConfig:
		return value.ID
	case *model.Account:
		return value.ID
	case *model.Source:
		return value.ID
	case *model.Sink:
		return value.ID
	case *model.Template:
		return value.ID
	case *model.Filter:
		return value.ID
	case *model.Flow:
		return value.ID
	case *model.AIDigestProfile:
		return value.ID
	case *model.AIDigestOutputTemplate:
		return value.ID
	}
	return 0
}
func mappedPtr(value *int64, mapping map[int64]int64) *int64 {
	if value == nil {
		return nil
	}
	mapped, ok := mapping[*value]
	if !ok {
		return nil
	}
	return &mapped
}
func remapJSONIDs(value datatypes.JSON, mapping map[int64]int64) datatypes.JSON {
	var ids []int64
	if json.Unmarshal(value, &ids) != nil {
		return value
	}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if mapped, ok := mapping[id]; ok {
			out = append(out, mapped)
		}
	}
	data, _ := json.Marshal(out)
	return datatypes.JSON(data)
}
