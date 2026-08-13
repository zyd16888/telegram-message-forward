package repository

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"

	domainsink "telegram-message-forward/internal/domain/sink"
	"telegram-message-forward/internal/infra/crypto"
	"telegram-message-forward/internal/storage/model"
)

func newTestSinkRepository(t *testing.T) *SinkRepository {
	t.Helper()
	cipher, err := crypto.NewCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	return NewSinkRepository(nil, cipher)
}

func TestSinkConfigStoredOnlyAsCiphertext(t *testing.T) {
	repo := newTestSinkRepository(t)
	sink := &domainsink.Sink{
		ID: 1, Type: "webhook", Name: "private config", Enabled: true,
		Config: map[string]any{"url": "https://example.com/hook?token=secret", "header": "private"},
		Secret: []byte("bearer-secret"),
	}
	m, err := repo.toModel(sink)
	if err != nil {
		t.Fatal(err)
	}
	if string(m.Config) != "{}" || len(m.ConfigEncrypted) == 0 {
		t.Fatalf("明文 config 不应落库: config=%s encrypted=%d", m.Config, len(m.ConfigEncrypted))
	}
	serialized, _ := json.Marshal(sink.Config)
	if string(m.ConfigEncrypted) == string(serialized) {
		t.Fatal("config_encrypted 不应等于明文 JSON")
	}
	roundTrip, err := repo.toDomain(m)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Config["url"] != sink.Config["url"] || string(roundTrip.Secret) != "bearer-secret" {
		t.Fatalf("加密配置往返不一致: %+v", roundTrip)
	}
}

func TestSinkLegacyPlainConfigCanStillBeRead(t *testing.T) {
	repo := newTestSinkRepository(t)
	secret, err := repo.cipher.Encrypt([]byte("old-secret"))
	if err != nil {
		t.Fatal(err)
	}
	m := &model.Sink{
		ID: 2, Type: "webhook", Config: datatypes.JSON([]byte(`{"url":"https://example.com/legacy"}`)),
		SecretEncrypted: secret,
	}
	sink, err := repo.toDomain(m)
	if err != nil {
		t.Fatal(err)
	}
	if sink.Config["url"] != "https://example.com/legacy" || string(sink.Secret) != "old-secret" {
		t.Fatalf("旧明文配置读取失败: %+v", sink)
	}
}
