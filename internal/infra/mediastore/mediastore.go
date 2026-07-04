// Package mediastore 提供媒体文件存储抽象：本地目录存储与 S3 兼容对象存储。
//
// Source 下载的媒体先落在临时目录，由 ingest 通过 PutFile 收编进存储层；
// worker 投递时通过 PublicURL 为需要公网 URL 的渠道生成可访问地址。
package mediastore

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Store 是媒体存储接口。
type Store interface {
	// PutFile 将本地文件 srcPath 以 key 纳入存储（移动语义），返回纳入后的本地缓存路径。
	// 对象存储实现上传失败时仍返回有效的本地路径与错误，调用方可降级继续使用本地文件。
	PutFile(ctx context.Context, key, srcPath string) (string, error)
	// PublicURL 返回 key 的公网可访问 URL；无法提供时返回空串（不视为错误）。
	PublicURL(ctx context.Context, key string) (string, error)
	// Open 打开 key 对应的内容。
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	// Cleanup 清理本地缓存中早于 olderThan 的文件，返回删除数量。
	// 对象存储侧的过期清理交由桶生命周期规则处理。
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
}

// ValidKey 校验存储键：必须是相对的斜杠路径，不允许目录穿越。
func ValidKey(key string) bool {
	if key == "" || strings.Contains(key, "\\") {
		return false
	}
	cleaned := path.Clean(key)
	if cleaned != key || strings.HasPrefix(cleaned, "/") || strings.HasPrefix(cleaned, "..") {
		return false
	}
	return true
}

// moveFile 移动文件；跨卷 rename 失败时退化为复制后删除源文件。
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("复制文件失败: %w", err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	in.Close()
	_ = os.Remove(src)
	return nil
}

// cleanupDir 删除 dir 下修改时间早于 cutoff 的文件，并顺带清理空目录。
func cleanupDir(dir string, cutoff time.Time) (int, error) {
	removed := 0
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			if rerr := os.Remove(p); rerr == nil {
				removed++
			}
		}
		return nil
	})
	if err != nil {
		return removed, err
	}
	// 自底向上清理空目录（根目录保留）。
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() || p == dir {
			return nil
		}
		_ = os.Remove(p) // 非空目录会失败，忽略即可
		return nil
	})
	return removed, nil
}
