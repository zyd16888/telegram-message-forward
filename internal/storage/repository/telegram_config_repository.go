package repository

import (
	"context"
	"fmt"

	domainconfig "telegram-message-forward/internal/domain/telegramconfig"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"

	"gorm.io/gorm"
)

// TelegramAppRepository 是 telegram_apps 的 PostgreSQL 实现。
type TelegramAppRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewTelegramAppRepository 创建 Telegram App 仓储。
func NewTelegramAppRepository(db *gorm.DB, cipher *crypto.Cipher) *TelegramAppRepository {
	return &TelegramAppRepository{db: db, cipher: cipher}
}

var _ domainconfig.TelegramAppRepository = (*TelegramAppRepository)(nil)

func (r *TelegramAppRepository) Create(ctx context.Context, app *domainconfig.TelegramApp) error {
	m, err := r.appToModel(app)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	app.ID = m.ID
	app.CreatedAt = m.CreatedAt
	app.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *TelegramAppRepository) Update(ctx context.Context, app *domainconfig.TelegramApp) error {
	m, err := r.appToModel(app)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *TelegramAppRepository) GetByID(ctx context.Context, id int64) (*domainconfig.TelegramApp, error) {
	var m model.TelegramApp
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return r.appToDomain(&m)
}

func (r *TelegramAppRepository) List(ctx context.Context) ([]*domainconfig.TelegramApp, error) {
	var ms []model.TelegramApp
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainconfig.TelegramApp, 0, len(ms))
	for i := range ms {
		app, err := r.appToDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, app)
	}
	return out, nil
}

func (r *TelegramAppRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.TelegramApp{}, id).Error
}

func (r *TelegramAppRepository) appToModel(app *domainconfig.TelegramApp) (*model.TelegramApp, error) {
	hashEnc, err := r.cipher.Encrypt([]byte(app.AppHash))
	if err != nil {
		return nil, fmt.Errorf("加密 app_hash 失败: %w", err)
	}
	return &model.TelegramApp{
		ID:               app.ID,
		Name:             app.Name,
		AppID:            app.AppID,
		AppHashEncrypted: hashEnc,
		Enabled:          app.Enabled,
		CreatedAt:        app.CreatedAt,
		UpdatedAt:        app.UpdatedAt,
	}, nil
}

func (r *TelegramAppRepository) appToDomain(m *model.TelegramApp) (*domainconfig.TelegramApp, error) {
	hash, err := r.cipher.Decrypt(m.AppHashEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密 app_hash 失败: %w", err)
	}
	return &domainconfig.TelegramApp{
		ID:        m.ID,
		Name:      m.Name,
		AppID:     m.AppID,
		AppHash:   string(hash),
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}

// ProxyConfigRepository 是 proxy_configs 的 PostgreSQL 实现。
type ProxyConfigRepository struct {
	db     *gorm.DB
	cipher *crypto.Cipher
}

// NewProxyConfigRepository 创建代理配置仓储。
func NewProxyConfigRepository(db *gorm.DB, cipher *crypto.Cipher) *ProxyConfigRepository {
	return &ProxyConfigRepository{db: db, cipher: cipher}
}

var _ domainconfig.ProxyRepository = (*ProxyConfigRepository)(nil)

func (r *ProxyConfigRepository) Create(ctx context.Context, p *domainconfig.Proxy) error {
	m, err := r.proxyToModel(p)
	if err != nil {
		return err
	}
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return err
	}
	p.ID = m.ID
	p.CreatedAt = m.CreatedAt
	p.UpdatedAt = m.UpdatedAt
	return nil
}

func (r *ProxyConfigRepository) Update(ctx context.Context, p *domainconfig.Proxy) error {
	m, err := r.proxyToModel(p)
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Save(m).Error
}

func (r *ProxyConfigRepository) GetByID(ctx context.Context, id int64) (*domainconfig.Proxy, error) {
	var m model.ProxyConfig
	if err := r.db.WithContext(ctx).First(&m, id).Error; err != nil {
		return nil, err
	}
	return r.proxyToDomain(&m)
}

func (r *ProxyConfigRepository) List(ctx context.Context) ([]*domainconfig.Proxy, error) {
	var ms []model.ProxyConfig
	if err := r.db.WithContext(ctx).Order("id").Find(&ms).Error; err != nil {
		return nil, err
	}
	out := make([]*domainconfig.Proxy, 0, len(ms))
	for i := range ms {
		p, err := r.proxyToDomain(&ms[i])
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (r *ProxyConfigRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&model.ProxyConfig{}, id).Error
}

func (r *ProxyConfigRepository) proxyToModel(p *domainconfig.Proxy) (*model.ProxyConfig, error) {
	passEnc, err := r.cipher.Encrypt([]byte(p.Password))
	if err != nil {
		return nil, fmt.Errorf("加密代理密码失败: %w", err)
	}
	return &model.ProxyConfig{
		ID:                p.ID,
		Name:              p.Name,
		Type:              p.Type,
		Addr:              p.Addr,
		Username:          p.Username,
		PasswordEncrypted: passEnc,
		Enabled:           p.Enabled,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}, nil
}

func (r *ProxyConfigRepository) proxyToDomain(m *model.ProxyConfig) (*domainconfig.Proxy, error) {
	pass, err := r.cipher.Decrypt(m.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("解密代理密码失败: %w", err)
	}
	return &domainconfig.Proxy{
		ID:        m.ID,
		Name:      m.Name,
		Type:      m.Type,
		Addr:      m.Addr,
		Username:  m.Username,
		Password:  string(pass),
		Enabled:   m.Enabled,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}, nil
}
