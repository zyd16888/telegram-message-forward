package api

import (
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"telegram-message-forward/internal/webui"
)

func mountWebStatic(r *gin.Engine, webDir string, log *slog.Logger) {
	staticFS, source, ok := selectWebStaticFS(webDir, log)
	if !ok {
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

		if fileName, ok := resolveStaticFileName(reqPath); ok {
			if info, err := fs.Stat(staticFS, fileName); err == nil && !info.IsDir() {
				serveStaticFile(c, staticFS, fileName)
				return
			}
		}

		serveStaticFile(c, staticFS, "index.html")
	})

	if log != nil {
		log.Info("前端静态文件托管已启用", "source", source)
	}
}

func selectWebStaticFS(webDir string, log *slog.Logger) (fs.FS, string, bool) {
	webDir = strings.TrimSpace(webDir)
	missingWebDir := ""
	if webDir != "" {
		dirFS := osDirFS(webDir)
		if hasIndexFile(dirFS) {
			return dirFS, webDir, true
		}
		missingWebDir = webDir
	}

	embeddedFS, ok := webui.FS()
	if ok {
		if log != nil && missingWebDir != "" {
			log.Info("前端构建目录不可用，改用内置静态资源", "web_dir", missingWebDir)
		}
		return embeddedFS, "embedded", true
	}

	if log != nil {
		if missingWebDir != "" {
			log.Warn("前端构建目录不可用，跳过静态托管", "web_dir", missingWebDir)
		} else {
			log.Warn("前端静态资源不可用，跳过静态托管")
		}
	}
	return nil, "", false
}

func osDirFS(dir string) fs.FS {
	return os.DirFS(filepath.Clean(dir))
}

func hasIndexFile(staticFS fs.FS) bool {
	info, err := fs.Stat(staticFS, "index.html")
	return err == nil && !info.IsDir()
}

func serveStaticFile(c *gin.Context, staticFS fs.FS, fileName string) {
	data, err := fs.ReadFile(staticFS, fileName)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	contentType := mime.TypeByExtension(path.Ext(fileName))
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	c.Data(http.StatusOK, contentType, data)
}

func isBackendPath(reqPath string) bool {
	return reqPath == "/healthz" || reqPath == "/api" || strings.HasPrefix(reqPath, "/api/")
}

func resolveStaticFileName(reqPath string) (string, bool) {
	cleanPath := path.Clean("/" + strings.TrimPrefix(reqPath, "/"))
	if cleanPath == "/" {
		return "", false
	}

	rel := strings.TrimPrefix(cleanPath, "/")
	if !fs.ValidPath(rel) {
		return "", false
	}

	return rel, true
}
