// Package httpclient 提供带超时的 HTTP 客户端封装，供 Sink 与外部适配层复用。
package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Client 是带默认超时的 HTTP 客户端封装。
type Client struct {
	hc *http.Client
}

// Option 配置 Client。
type Option func(*Client)

// WithTimeout 设置请求超时。
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.hc.Timeout = d
	}
}

// WithHTTPClient 注入自定义 http.Client（如带代理）。
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.hc = hc
		}
	}
}

// New 创建 Client，默认超时 30s。
func New(opts ...Option) *Client {
	c := &Client{hc: &http.Client{Timeout: 30 * time.Second}}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Response 是一次 HTTP 调用的结果。
type Response struct {
	StatusCode int
	Body       []byte
}

// IsSuccess 返回 2xx 是否成立。
func (r *Response) IsSuccess() bool {
	return r.StatusCode >= 200 && r.StatusCode < 300
}

// PostJSON 发送 JSON body 并返回响应。body 可以是任意可序列化对象或 []byte。
func (c *Client) PostJSON(ctx context.Context, url string, body any, headers map[string]string) (*Response, error) {
	var payload []byte
	switch v := body.(type) {
	case nil:
		payload = nil
	case []byte:
		payload = v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		payload = b
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(req)
}

// Get 发送 GET 请求。
func (c *Client) Get(ctx context.Context, url string, headers map[string]string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(req)
}

// PostMultipartFile 上传单个文件字段并返回响应。
func (c *Client) PostMultipartFile(ctx context.Context, url, fieldName, path string, headers map[string]string) (*Response, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("打开上传文件失败: %w", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile(fieldName, filepath.Base(path))
	if err != nil {
		return nil, fmt.Errorf("创建 multipart 字段失败: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("写入 multipart 文件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("关闭 multipart body 失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return nil, fmt.Errorf("构建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) (*Response, error) {
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}
	return &Response{StatusCode: resp.StatusCode, Body: data}, nil
}
