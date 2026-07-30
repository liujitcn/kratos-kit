// Package zookeeper 提供基于 ZooKeeper znode 的 Kratos 配置源。
package zookeeper

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-zookeeper/zk"
)

// Client 定义配置源使用的 ZooKeeper 客户端能力。
type Client interface {
	Get(path string) ([]byte, *zk.Stat, error)
	GetW(path string) ([]byte, *zk.Stat, <-chan zk.Event, error)
	ExistsW(path string) (bool, *zk.Stat, <-chan zk.Event, error)
}

// Option 配置 ZooKeeper 配置源。
type Option func(*options)

type options struct {
	ctx    context.Context
	path   string
	format string
}

// WithContext 设置 Watcher 使用的父 context。
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		}
	}
}

// WithPath 设置保存配置内容的 znode 路径。
func WithPath(path string) Option {
	return func(o *options) {
		o.path = path
	}
}

// WithFormat 显式设置配置格式。
func WithFormat(format string) Option {
	return func(o *options) {
		o.format = strings.TrimPrefix(format, ".")
	}
}

// New 创建 ZooKeeper 配置源。
func New(client Client, opts ...Option) (config.Source, error) {
	if client == nil {
		return nil, errors.New("config/zookeeper: client is nil")
	}

	cfg := &options{ctx: context.Background()}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.path == "" {
		return nil, errors.New("config/zookeeper: path is empty")
	}
	return &source{
		client:  client,
		options: cfg,
	}, nil
}

type source struct {
	client  Client
	options *options
}

// Load 获取 znode 当前保存的配置。
func (s *source) Load() ([]*config.KeyValue, error) {
	value, _, err := s.client.Get(s.options.path)
	if errors.Is(err, zk.ErrNoNode) {
		return []*config.KeyValue{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("config/zookeeper: get %s: %w", s.options.path, err)
	}
	return []*config.KeyValue{s.keyValue(value)}, nil
}

// Watch 创建不会重复推送初始值的一次性 watch 管理器。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(s.options.ctx)
	w := &watcher{
		source: s,
		ctx:    ctx,
		cancel: cancel,
	}
	err := w.arm()
	if err != nil {
		cancel()
		return nil, err
	}
	return w, nil
}

// keyValue 将 znode 内容转换为 Kratos KeyValue。
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
	events <-chan zk.Event
}

// Next 等待 znode 创建或内容变更，并自动重建一次性 watch。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, w.ctx.Err()
		case event, ok := <-w.events:
			if !ok {
				return nil, errors.New("config/zookeeper: watch closed")
			}
			if event.Err != nil {
				return nil, fmt.Errorf("config/zookeeper: watch %s: %w", w.source.options.path, event.Err)
			}

			value, found, err := w.reload()
			if err != nil {
				return nil, err
			}
			if !found {
				continue
			}
			return []*config.KeyValue{w.source.keyValue(value)}, nil
		}
	}
}

// reload 在事件后读取最新值并重新注册一次性 watch。
func (w *watcher) reload() ([]byte, bool, error) {
	value, _, events, err := w.source.client.GetW(w.source.options.path)
	if errors.Is(err, zk.ErrNoNode) {
		err = w.arm()
		if err != nil {
			return nil, false, err
		}
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("config/zookeeper: get and watch %s: %w", w.source.options.path, err)
	}
	w.events = events
	return value, true, nil
}

// arm 根据 znode 是否存在注册数据或创建 watch。
func (w *watcher) arm() error {
	_, _, events, err := w.source.client.GetW(w.source.options.path)
	if err == nil {
		w.events = events
		return nil
	}
	if !errors.Is(err, zk.ErrNoNode) {
		return fmt.Errorf("config/zookeeper: watch %s: %w", w.source.options.path, err)
	}

	_, _, events, err = w.source.client.ExistsW(w.source.options.path)
	if err != nil {
		return fmt.Errorf("config/zookeeper: watch existence %s: %w", w.source.options.path, err)
	}
	w.events = events
	return nil
}

// Stop 停止 ZooKeeper Watcher。
func (w *watcher) Stop() error {
	w.cancel()
	return nil
}
