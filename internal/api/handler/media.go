package handler

import (
	"context"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// MediaFileStore 是媒体端点所需的最小存储能力（由本地媒体存储实现）。
type MediaFileStore interface {
	VerifySignedPath(key string, expires int64, sig string) bool
	Open(ctx context.Context, key string) (io.ReadCloser, error)
}

// MediaHandler 对外提供带签名校验的媒体文件访问。
type MediaHandler struct {
	store MediaFileStore
}

// NewMediaHandler 创建媒体 handler。
func NewMediaHandler(store MediaFileStore) *MediaHandler {
	return &MediaHandler{store: store}
}

// Serve 处理 GET /media/*key：校验 HMAC 签名与有效期后返回文件内容。
// 该端点不走管理后台鉴权——下游渠道（钉钉、Bark 等）的服务器需要能直接拉取。
func (h *MediaHandler) Serve(c *gin.Context) {
	key := strings.TrimPrefix(c.Param("key"), "/")
	expires, err := strconv.ParseInt(c.Query("e"), 10, 64)
	if err != nil || !h.store.VerifySignedPath(key, expires, c.Query("s")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "签名无效或已过期"})
		return
	}

	rc, err := h.store.Open(c.Request.Context(), key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体不存在"})
		return
	}
	defer rc.Close()

	contentType := mime.TypeByExtension(path.Ext(key))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Status(http.StatusOK)
	_, _ = io.Copy(c.Writer, rc)
}
