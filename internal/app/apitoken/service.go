// Package apitoken 提供管理 API token 的生成、查询与吊销。
package apitoken

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	domainapitoken "telegram-message-forward/internal/domain/apitoken"
	"telegram-message-forward/internal/security"
)

// Service 是 API token 应用服务。
type Service struct {
	repo domainapitoken.Repository
}

// NewService 创建 token 服务。
func NewService(repo domainapitoken.Repository) *Service {
	return &Service{repo: repo}
}

// Created 是新建 token 的结果，明文仅此一次返回。
type Created struct {
	ID    int64
	Name  string
	Token string
}

// Create 生成一个新 token，存哈希，返回明文（仅此一次）。
func (s *Service) Create(ctx context.Context, name string) (*Created, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return nil, err
	}
	token := hex.EncodeToString(buf)
	id, err := s.repo.Create(ctx, name, security.HashToken(token))
	if err != nil {
		return nil, err
	}
	return &Created{ID: id, Name: name, Token: token}, nil
}

// List 返回全部 token（不含明文）。
func (s *Service) List(ctx context.Context) ([]*domainapitoken.Token, error) {
	return s.repo.List(ctx)
}

// Revoke 吊销一个 token。
func (s *Service) Revoke(ctx context.Context, id int64) error {
	return s.repo.Revoke(ctx, id)
}
