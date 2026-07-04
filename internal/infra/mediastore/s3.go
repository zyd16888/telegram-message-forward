package mediastore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Options 是 S3 兼容对象存储的连接参数。
type S3Options struct {
	Endpoint      string
	Region        string
	Bucket        string
	AccessKey     string
	SecretKey     string
	UseSSL        bool
	KeyPrefix     string
	PublicBaseURL string
	URLTTL        time.Duration
	// AutoCleanup 开启后 Cleanup 会同步删除对象存储中超过保留期的对象。
	// 与其他数据共用一个桶时请务必设置 KeyPrefix，否则会误删桶内无关对象。
	AutoCleanup bool
}

// S3 是 S3 兼容对象存储实现（AWS S3 / Cloudflare R2 / MinIO / OSS / COS 等）。
//
// 本地目录仍作为二进制缓存保留：需要直接读取文件内容的渠道
//（企业微信 base64、邮件附件等）继续走本地路径。
type S3 struct {
	local  *Local
	client *minio.Client
	opts   S3Options
}

// NewS3 创建 S3 存储。
func NewS3(opts S3Options, local *Local) (*S3, error) {
	if opts.URLTTL <= 0 {
		opts.URLTTL = 24 * time.Hour
	}
	opts.PublicBaseURL = strings.TrimRight(opts.PublicBaseURL, "/")
	opts.KeyPrefix = strings.Trim(opts.KeyPrefix, "/")
	client, err := newS3Client(opts)
	if err != nil {
		return nil, err
	}
	return &S3{local: local, client: client, opts: opts}, nil
}

func newS3Client(opts S3Options) (*minio.Client, error) {
	if opts.Endpoint == "" || opts.Bucket == "" {
		return nil, fmt.Errorf("media.s3 缺少 endpoint 或 bucket")
	}
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.UseSSL,
		Region: opts.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("初始化 S3 客户端失败: %w", err)
	}
	return client, nil
}

// TestS3 校验 S3 连通性与桶可访问性，供设置页「测试连接」使用。
func TestS3(ctx context.Context, opts S3Options) error {
	client, err := newS3Client(opts)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	exists, err := client.BucketExists(ctx, opts.Bucket)
	if err != nil {
		return fmt.Errorf("连接 S3 失败: %w", err)
	}
	if !exists {
		return fmt.Errorf("桶 %q 不存在或无访问权限", opts.Bucket)
	}
	return nil
}

var _ Store = (*S3)(nil)

// PutFile 先纳入本地缓存，再上传对象存储。
// 上传失败时返回本地路径与错误：调用方可保留本地文件降级使用。
func (s *S3) PutFile(ctx context.Context, key, srcPath string) (string, error) {
	localPath, err := s.local.PutFile(ctx, key, srcPath)
	if err != nil {
		return "", err
	}
	contentType := mime.TypeByExtension(path.Ext(key))
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err = s.client.FPutObject(ctx, s.opts.Bucket, s.objectKey(key), localPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return localPath, fmt.Errorf("上传对象存储失败: %w", err)
	}
	return localPath, nil
}

// PublicURL 返回对象的公网 URL：配置了 public_base_url（公开桶或 CDN）
// 时直接拼接，否则生成预签名 URL。
func (s *S3) PublicURL(ctx context.Context, key string) (string, error) {
	if !ValidKey(key) {
		return "", fmt.Errorf("非法存储键: %q", key)
	}
	if s.opts.PublicBaseURL != "" {
		return s.opts.PublicBaseURL + "/" + escapeKeyPath(s.objectKey(key)), nil
	}
	u, err := s.client.PresignedGetObject(ctx, s.opts.Bucket, s.objectKey(key), s.opts.URLTTL, url.Values{})
	if err != nil {
		return "", fmt.Errorf("生成预签名 URL 失败: %w", err)
	}
	return u.String(), nil
}

// Open 优先读本地缓存，缺失时回源对象存储。
func (s *S3) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	if rc, err := s.local.Open(ctx, key); err == nil {
		return rc, nil
	}
	obj, err := s.client.GetObject(ctx, s.opts.Bucket, s.objectKey(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// Cleanup 清理本地缓存；开启 AutoCleanup 时同步删除对象存储中过期的对象，
// 否则对象存储侧交由桶生命周期规则处理。
func (s *S3) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	removed, localErr := s.local.Cleanup(ctx, olderThan)
	if !s.opts.AutoCleanup {
		return removed, localErr
	}

	cutoff := time.Now().Add(-olderThan)
	prefix := ""
	if s.opts.KeyPrefix != "" {
		prefix = s.opts.KeyPrefix + "/"
	}
	var remoteErr error
	for obj := range s.client.ListObjects(ctx, s.opts.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	}) {
		if obj.Err != nil {
			remoteErr = fmt.Errorf("遍历对象存储失败: %w", obj.Err)
			break
		}
		if obj.LastModified.IsZero() || !obj.LastModified.Before(cutoff) {
			continue
		}
		if err := s.client.RemoveObject(ctx, s.opts.Bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			remoteErr = fmt.Errorf("删除过期对象失败: %w", err)
			continue
		}
		removed++
	}
	return removed, errors.Join(localErr, remoteErr)
}

func (s *S3) objectKey(key string) string {
	if s.opts.KeyPrefix == "" {
		return key
	}
	return s.opts.KeyPrefix + "/" + key
}
