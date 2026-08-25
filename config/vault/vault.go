package vault

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/hashicorp/vault/api"
)

const defaultPollInterval = 30 * time.Second

// Option 配置 Vault 配置源。
type Option func(*options)

type options struct {
	ctx          context.Context
	path         string
	dataKey      string
	pollInterval time.Duration
	format       string
}

// WithContext 设置 Vault 请求使用的父 context。
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

// WithPath 设置 Vault Secret 路径。
func WithPath(path string) Option {
	return func(o *options) {
		o.path = path
	}
}

// WithDataKey 设置 Secret Data 内保存配置内容的字段名。
func WithDataKey(key string) Option {
	return func(o *options) {
		if key != "" {
			o.dataKey = key
		}
	}
}

// WithPollInterval 设置 Vault 变更轮询间隔。
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

// New 创建 Vault 配置源。
func New(client *api.Client, opts ...Option) (config.Source, error) {
	if client == nil {
		return nil, errors.New("config/vault: client is nil")
	}

	cfg := &options{
		ctx:          context.Background(),
		dataKey:      "content",
		pollInterval: defaultPollInterval,
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.path == "" {
		return nil, errors.New("config/vault: path is empty")
	}
	return &source{
		client:  client,
		options: cfg,
	}, nil
}

type source struct {
	mu        sync.Mutex
	client    *api.Client
	options   *options
	lastValue []byte
}

// Load 获取 Vault 当前配置内容。
func (s *source) Load() ([]*config.KeyValue, error) {
	value, found, err := s.load(s.options.ctx)
	if err != nil {
		return nil, err
	}
	if !found {
		return []*config.KeyValue{}, nil
	}

	s.mu.Lock()
	s.lastValue = bytes.Clone(value)
	s.mu.Unlock()
	return []*config.KeyValue{s.keyValue(value)}, nil
}

// Watch 创建 Vault 轮询 Watcher。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(s.options.ctx)
	return &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		ticker: time.NewTicker(s.options.pollInterval),
	}, nil
}

// load 读取并展开 Vault KV v1 或 KV v2 Secret。
func (s *source) load(ctx context.Context) ([]byte, bool, error) {
	secret, err := s.client.Logical().ReadWithContext(ctx, s.options.path)
	if err != nil {
		return nil, false, fmt.Errorf("config/vault: read %s: %w", s.options.path, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, false, nil
	}

	value, err := extractValue(secret.Data, s.options.dataKey)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// keyValue 将 Vault 内容转换为 Kratos KeyValue。
func (s *source) keyValue(value []byte) *config.KeyValue {
	format := s.options.format
	if format == "" {
		format = strings.TrimPrefix(filepath.Ext(s.options.path), ".")
	}
	return &config.KeyValue{
		Key:    s.options.path,
		Value:  value,
		Format: format,
	}
}

type watcher struct {
	source *source
	ctx    context.Context
	cancel context.CancelFunc
	ticker *time.Ticker
}

// Next 等待 Vault 配置内容发生变化。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case <-w.ticker.C:
			value, found, err := w.source.load(w.ctx)
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}

			w.source.mu.Lock()
			if bytes.Equal(value, w.source.lastValue) {
				w.source.mu.Unlock()
				continue
			}
			w.source.lastValue = bytes.Clone(value)
			w.source.mu.Unlock()
			return []*config.KeyValue{w.source.keyValue(value)}, nil
		}
	}
}

// Stop 停止 Vault 配置轮询。
func (w *watcher) Stop() error {
	w.ticker.Stop()
	w.cancel()
	return nil
}

// extractValue 从 Vault Secret Data 中提取配置内容。
func extractValue(data map[string]any, dataKey string) ([]byte, error) {
	if inner, ok := data["data"].(map[string]any); ok {
		data = inner
	}

	value, ok := data[dataKey]
	if !ok {
		return json.Marshal(data)
	}
	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return typed, nil
	default:
		return json.Marshal(typed)
	}
}
