// Package fs 提供基于 io/fs.FS 的 Kratos 静态配置源。
package fs

import (
	"context"
	"errors"
	"fmt"
	iofs "io/fs"
	"path"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
)

// Option 配置文件系统配置源。
type Option func(*options)

type options struct {
	fsys iofs.FS
	path string
}

// WithFS 设置配置文件所在的文件系统。
func WithFS(fsys iofs.FS) Option {
	return func(o *options) {
		o.fsys = fsys
	}
}

// WithPath 设置文件系统内的配置文件路径。
func WithPath(filePath string) Option {
	return func(o *options) {
		o.path = filePath
	}
}

// New 创建文件系统配置源。
func New(opts ...Option) (config.Source, error) {
	cfg := &options{}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.fsys == nil {
		return nil, errors.New("config/fs: fs is nil")
	}
	if cfg.path == "" {
		return nil, errors.New("config/fs: path is empty")
	}
	return &source{options: cfg}, nil
}

type source struct {
	options *options
}

// Load 读取静态配置文件。
func (s *source) Load() ([]*config.KeyValue, error) {
	value, err := iofs.ReadFile(s.options.fsys, s.options.path)
	if err != nil {
		return nil, fmt.Errorf("config/fs: read %s: %w", s.options.path, err)
	}
	return []*config.KeyValue{{
		Key:    s.options.path,
		Value:  value,
		Format: strings.TrimPrefix(path.Ext(s.options.path), "."),
	}}, nil
}

// Watch 返回静态配置源的阻塞 Watcher。
func (s *source) Watch() (config.Watcher, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &watcher{
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

type watcher struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Next 阻塞到 Watcher 被停止。
func (w *watcher) Next() ([]*config.KeyValue, error) {
	<-w.ctx.Done()
	return nil, w.ctx.Err()
}

// Stop 停止静态配置 Watcher。
func (w *watcher) Stop() error {
	w.cancel()
	return nil
}
