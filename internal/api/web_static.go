package api

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func mountWebStatic(r *gin.Engine, webDir string, log *slog.Logger) {
	webDir = strings.TrimSpace(webDir)
	if webDir == "" {
		return
	}

	indexPath := filepath.Join(webDir, "index.html")
	if info, err := os.Stat(indexPath); err != nil || info.IsDir() {
		if log != nil {
			log.Warn("前端构建目录不可用，跳过静态托管", "web_dir", webDir, "err", err)
		}
		return
	}

	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		reqPath := c.Request.URL.Path
		if isBackendPath(reqPath) {
			c.Status(http.StatusNotFound)
			return
		}

		if filePath, ok := resolveStaticFile(webDir, reqPath); ok {
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				c.File(filePath)
				return
			}
		}

		c.File(indexPath)
	})

	if log != nil {
		log.Info("前端静态文件托管已启用", "web_dir", webDir)
	}
}

func isBackendPath(reqPath string) bool {
	return reqPath == "/healthz" || reqPath == "/api" || strings.HasPrefix(reqPath, "/api/")
}

func resolveStaticFile(root, reqPath string) (string, bool) {
	cleanPath := path.Clean("/" + strings.TrimPrefix(reqPath, "/"))
	if cleanPath == "/" {
		return "", false
	}

	rel := strings.TrimPrefix(cleanPath, "/")
	target := filepath.Join(root, filepath.FromSlash(rel))

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", false
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	relToRoot, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil || relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(os.PathSeparator)) {
		return "", false
	}

	return targetAbs, true
}
