// Package security 提供 API 鉴权相关的哈希与校验。
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// GenerateToken 生成随机明文 token。调用方只应在响应中显示一次，存储层保存哈希。
func GenerateToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// HashToken 返回 token 的十六进制 SHA-256 哈希。数据库只存哈希，不存明文。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TokenValidator 基于哈希查找校验 API token。
type TokenValidator struct {
	// Lookup 判断给定 token 哈希是否为有效（存在且未吊销）的 token。
	Lookup func(hash string) (bool, error)
}

// Validate 实现 api/middleware.TokenValidator。
func (v TokenValidator) Validate(token string) bool {
	if v.Lookup == nil {
		return false
	}
	ok, err := v.Lookup(HashToken(token))
	if err != nil {
		return false
	}
	return ok
}
