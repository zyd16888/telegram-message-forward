// Package crypto 收口敏感字段的加解密。
//
// 使用 AES-256-GCM 加密 Telegram session、sink secret、proxy 凭据、app_hash 等。
// 主密钥（KEK）来自配置或环境变量，其它层只接触密文或解密后的内存值。
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey 表示主密钥长度不合法。
var ErrInvalidKey = errors.New("加密主密钥必须为 32 字节")

// Cipher 封装 AES-256-GCM 加解密。
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher 用 32 字节主密钥构建 Cipher。
func NewCipher(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("构建 AES cipher 失败: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("构建 GCM 失败: %w", err)
	}
	return &Cipher{aead: aead}, nil
}

// Encrypt 加密明文，返回 nonce||ciphertext。plaintext 为 nil 时返回 nil。
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	if plaintext == nil {
		return nil, nil
	}
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成 nonce 失败: %w", err)
	}
	return c.aead.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt 解密 nonce||ciphertext。ciphertext 为 nil 时返回 nil。
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if ciphertext == nil {
		return nil, nil
	}
	nonceSize := c.aead.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("密文长度不足")
	}
	nonce, payload := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := c.aead.Open(nil, nonce, payload, nil)
	if err != nil {
		return nil, fmt.Errorf("解密失败: %w", err)
	}
	return plaintext, nil
}
