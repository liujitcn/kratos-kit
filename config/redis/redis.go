// Package redis 提供基于 Redis Key 和 Pub/Sub 的 Kratos 配置源。
package redis

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	goredis "github.com/redis/go-redis/v9"
)

const channelPrefix = "__kratos_config__:"

// Option 配置 Redis 配置源。
type Option func(*options)

type options struct {
	ctx     context.Context
	path    string
	channel string
	format  string
}

// WithContext 设置 Redis 操作使用的父 context。
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

// WithPath 设置保存配置内容的 Redis Key。
func WithPath(path string) Option {
	return func(o *options) {
		o.path = path
	}
}

// WithChannel 设置接收配置变更通知的 Pub/Sub Channel。
func WithChannel(channel string) Option {
	return func(o *options) {
		o.channel = channel
	}
}

// WithFormat 显式设置配置格式。
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = strings.TrimPrefix(format, ".")
	}
}

// New 创建 Redis 配置源。
func New(client goredis.UniversalClient, opts ...Option) (config.Source, error) {
	if client == nil {
		return nil, errors.New("config/redis: client is nil")
	}

	cfg := &options{ctx: context.Background()}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.path == "" {
		return nil, errors.New("config/redis: path is empty")
	}
	if cfg.channel == "" {
		cfg.channel = channelPrefix + cfg.path
	}
	return &source{
		client:  client,
		options: cfg,
	}, nil
}

type source struct {
	client  goredis.UniversalClient
	options *options
}

// Load 获取 Redis Key 当前保存的配置。
func (s *source) Load() ([]*config.KeyValue, error) {
	return s.load(s.options.ctx)
}

// Watch 订阅配置变更 Channel。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(s.options.ctx)
	pubsub := s.client.Subscribe(ctx, s.options.channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		cancel()
		_ = pubsub.Close()
		return nil, fmt.Errorf("config/redis: subscribe %s: %w", s.options.channel, err)
	}
	return &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
		pubsub: pubsub,
		msgs:   pubsub.Channel(),
	}, nil
}

// load 读取配置并转换为 Kratos KeyValue。
func (s *source) load(ctx context.Context) ([]*config.KeyValue, error) {
	value, err := s.client.Get(ctx, s.options.path).Bytes()
	if err == goredis.Nil {
		return []*config.KeyValue{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config/redis: get %s: %w", s.options.path, err)
	}
	return []*config.KeyValue{s.keyValue(value)}, nil
}

// keyValue 将 Redis Value 转换为 Kratos KeyValue。
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
	pubsub *goredis.PubSub
	msgs   <-chan *goredis.Message
}

// Next 等待下一条 Redis 配置变更通知。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	select {
	case <-w.ctx.Done():
		return nil, w.ctx.Err()
	case message, ok := <-w.msgs:
		if !ok {
			return nil, errors.New("config/redis: subscription closed")
		}
		if message.Payload != "" {
			return []*config.KeyValue{w.source.keyValue([]byte(message.Payload))}, nil
		}
		return w.source.load(w.ctx)
	}
}

// Stop 停止 Redis Pub/Sub Watcher。
func (w *watcher) Stop() error {
	w.cancel()
	return w.pubsub.Close()
}
