// Package http 提供支持条件轮询的 Kratos HTTP 配置源。
package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/config"
)

const defaultPollInterval = 30 * time.Second

// Option 配置 HTTP 配置源。
type Option func(*options)

type options struct {
	ctx          context.Context
	url          string
	method       string
	headers      http.Header
	client       *http.Client
	pollInterval time.Duration
	format       string
}

// WithContext 设置 HTTP 请求使用的父 context。
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

// WithURL 设置配置地址。
func WithURL(endpoint string) Option {
	return func(o *options) {
		o.url = endpoint
	}
}

// WithMethod 设置 HTTP 请求方法。
func WithMethod(method string) Option {
	return func(o *options) {
		if method != "" {
			o.method = method
		}
	}
}

// WithHeader 添加每次请求携带的 HTTP Header。
func WithHeader(key, value string) Option {
	return func(o *options) {
		o.headers.Set(key, value)
	}
}

// WithHTTPClient 设置调用方管理的 HTTP Client。
func WithHTTPClient(client *http.Client) Option {
	return func(o *options) {
		if client != nil {
			o.client = client
		}
	}
}

// WithPollInterval 设置配置变更轮询间隔。
func WithPollInterval(interval time.Duration) Option {
	return func(o *options) {
		if interval > 0 {
			o.pollInterval = interval
		}
	}
}

// WithFormat 显式设置配置格式。
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = strings.TrimPrefix(format, ".")
	}
}

// New 创建 HTTP 配置源。
func New(opts ...Option) (config.Source, error) {
	cfg := &options{
		ctx:          context.Background(),
		method:       http.MethodGet,
		headers:      make(http.Header),
		client:       &http.Client{Timeout: 10 * time.Second},
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.url == "" {
		return nil, errors.New("config/http: url is empty")
	}
	if _, err := url.ParseRequestURI(cfg.url); err != nil {
		return nil, fmt.Errorf("config/http: invalid url: %w", err)
	}
	return &source{options: cfg}, nil
}

type source struct {
	mu       sync.Mutex
	options  *options
	lastETag string
	lastBody []byte
}

// Load 获取当前 HTTP 配置。
func (s *source) Load() ([]*config.KeyValue, error) {
	body, etag, _, err := s.fetch(s.options.ctx, "")
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.lastETag = etag
	s.lastBody = bytes.Clone(body)
	s.mu.Unlock()
	return []*config.KeyValue{s.keyValue(body)}, nil
}

// Watch 创建条件轮询 Watcher。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(s.options.ctx)
	return &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		ticker: time.NewTicker(s.options.pollInterval),
	}, nil
}

// fetch 发起 HTTP 条件请求。
func (s *source) fetch(ctx context.Context, etag string) ([]byte, string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, s.options.method, s.options.url, nil)
	if err != nil {
		return nil, "", false, fmt.Errorf("config/http: create request: %w", err)
	}
	req.Header = s.options.headers.Clone()
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	response, err := s.options.client.Do(req)
	if err != nil {
		return nil, "", false, fmt.Errorf("config/http: request %s: %w", s.options.url, err)
	}
	defer response.Body.Close()

	responseETag := response.Header.Get("ETag")
	if response.StatusCode == http.StatusNotModified {
		if responseETag == "" {
			responseETag = etag
		}
		return nil, responseETag, true, nil
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, responseETag, false, fmt.Errorf("config/http: request %s: %s", s.options.url, response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, responseETag, false, fmt.Errorf("config/http: read response: %w", err)
	}
	return body, responseETag, false, nil
}

// keyValue 将响应内容转换为 Kratos KeyValue。
func (s *source) keyValue(body []byte) *config.KeyValue {
	format := s.options.format
	if format == "" {
		endpoint, _ := url.Parse(s.options.url)
		format = strings.TrimPrefix(path.Ext(endpoint.Path), ".")
	}
	return &config.KeyValue{
		Key:    s.options.url,
		Value:  body,
		Format: format,
	}
}

type watcher struct {
	source *source
	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
}

// Next 等待下一次内容变化。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case <-w.ticker.C:
			w.source.mu.Lock()
			etag := w.source.lastETag
			lastBody := bytes.Clone(w.source.lastBody)
			w.source.mu.Unlock()

			body, responseETag, notModified, err := w.source.fetch(w.ctx, etag)
			if err != nil {
				return nil, err
			}
			if notModified || bytes.Equal(body, lastBody) {
				if responseETag != etag {
					w.source.mu.Lock()
					w.source.lastETag = responseETag
					w.source.mu.Unlock()
				}
				continue
			}

			w.source.mu.Lock()
			w.source.lastETag = responseETag
			w.source.lastBody = bytes.Clone(body)
			w.source.mu.Unlock()
			return []*config.KeyValue{w.source.keyValue(body)}, nil
		}
	}
}

// Stop 停止 HTTP 配置轮询。
func (w *watcher) Stop() error {
	w.ticker.Stop()
	w.cancel()
	return nil
}
