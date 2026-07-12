// Package backup 提供口令加密的配置备份用例。
package backup

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"

	domainbackup "telegram-message-forward/internal/domain/backup"
)

const (
	archiveMagic   = "TMF-CONFIG-BACKUP"
	maxArchiveSize = 20 << 20
	argonTime      = 2
	argonMemory    = 32 * 1024
	argonThreads   = 2
)

var (
	ErrWeakPassword    = errors.New("备份口令至少需要 8 个字符")
	ErrInvalidArchive  = errors.New("备份文件无效或已损坏")
	ErrArchiveTooLarge = errors.New("备份文件不能超过 20MB")
)

type envelope struct {
	Magic      string `json:"magic"`
	Version    int    `json:"version"`
	Salt       string `json:"salt"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type Service struct {
	repo domainbackup.Repository
}

func NewService(repo domainbackup.Repository) *Service { return &Service{repo: repo} }

func (s *Service) Export(ctx context.Context, password string, includeSessions bool) ([]byte, domainbackup.Manifest, error) {
	if len(password) < 8 {
		return nil, domainbackup.Manifest{}, ErrWeakPassword
	}
	payload, manifest, err := s.repo.ExportPayload(ctx, includeSessions)
	if err != nil {
		return nil, domainbackup.Manifest{}, err
	}
	archive, err := encrypt(payload, password)
	return archive, manifest, err
}

func (s *Service) Inspect(ctx context.Context, archive []byte, password string) (*domainbackup.Preview, error) {
	payload, err := decrypt(archive, password)
	if err != nil {
		return nil, err
	}
	return s.repo.InspectPayload(ctx, payload)
}

func (s *Service) Restore(ctx context.Context, archive []byte, password string) (*domainbackup.RestoreResult, error) {
	payload, err := decrypt(archive, password)
	if err != nil {
		return nil, err
	}
	preview, err := s.repo.InspectPayload(ctx, payload)
	if err != nil {
		return nil, err
	}
	if !preview.CanRestore {
		return nil, errors.New(preview.Warning)
	}
	return s.repo.RestorePayload(ctx, payload)
}

func ReadArchive(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxArchiveSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxArchiveSize {
		return nil, ErrArchiveTooLarge
	}
	return data, nil
}

func encrypt(plaintext []byte, password string) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, []byte(archiveMagic))
	return json.Marshal(envelope{
		Magic: archiveMagic, Version: domainbackup.FormatVersion,
		Salt: base64.RawStdEncoding.EncodeToString(salt), Nonce: base64.RawStdEncoding.EncodeToString(nonce),
		Ciphertext: base64.RawStdEncoding.EncodeToString(sealed),
	})
}

func decrypt(archive []byte, password string) ([]byte, error) {
	if len(password) < 8 {
		return nil, ErrWeakPassword
	}
	var env envelope
	if err := json.Unmarshal(archive, &env); err != nil || env.Magic != archiveMagic || env.Version != domainbackup.FormatVersion {
		return nil, ErrInvalidArchive
	}
	salt, err := base64.RawStdEncoding.DecodeString(env.Salt)
	if err != nil || len(salt) != 16 {
		return nil, ErrInvalidArchive
	}
	nonce, err := base64.RawStdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, ErrInvalidArchive
	}
	sealed, err := base64.RawStdEncoding.DecodeString(env.Ciphertext)
	if err != nil {
		return nil, ErrInvalidArchive
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidArchive
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil || len(nonce) != gcm.NonceSize() {
		return nil, ErrInvalidArchive
	}
	plain, err := gcm.Open(nil, nonce, sealed, []byte(archiveMagic))
	if err != nil {
		return nil, fmt.Errorf("%w：口令错误或文件被篡改", ErrInvalidArchive)
	}
	return plain, nil
}
