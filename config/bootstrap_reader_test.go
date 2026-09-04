package config

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	kratosconfig "github.com/go-kratos/kratos/v3/config"
	kratoslog "github.com/go-kratos/kratos/v3/log"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

type bootstrapTestSource struct {
	keyValues  []*kratosconfig.KeyValue
	watchCalls int
	watcher    *bootstrapTestWatcher
}

// Load 返回测试配置内容。
func (s *bootstrapTestSource) Load() ([]*kratosconfig.KeyValue, error) {
	return s.keyValues, nil
}

// Watch 记录正式配置是否启动 watcher。
func (s *bootstrapTestSource) Watch() (kratosconfig.Watcher, error) {
	s.watchCalls++
	s.watcher = newBootstrapTestWatcher()
	return s.watcher, nil
}

type bootstrapTestWatcher struct {
	done     chan struct{}
	updates  chan []*kratosconfig.KeyValue
	stopOnce sync.Once
}

// Next 阻塞到测试 watcher 被停止。
func (w *bootstrapTestWatcher) Next() ([]*kratosconfig.KeyValue, error) {
	select {
	case <-w.done:
		return nil, context.Canceled
	case keyValues := <-w.updates:
		return keyValues, nil
	}
}

// Stop 结束测试 watcher 的阻塞等待。
func (w *bootstrapTestWatcher) Stop() error {
	w.stopOnce.Do(func() {
		close(w.done)
	})
	return nil
}

// newBootstrapTestWatcher 创建可安全停止的测试 watcher。
func newBootstrapTestWatcher() *bootstrapTestWatcher {
	return &bootstrapTestWatcher{
		done:    make(chan struct{}),
		updates: make(chan []*kratosconfig.KeyValue),
	}
}

// TestLoadBootstrapConfigWithoutWatch 验证临时配置加载不会启动 watcher。
func TestLoadBootstrapConfigWithoutWatch(t *testing.T) {
	source := &bootstrapTestSource{keyValues: []*kratosconfig.KeyValue{
		{
			Key:    "config.yaml",
			Format: "yaml",
			Value:  []byte("config:\n  type: etcd\n  etcd:\n    key: kratos.bootstrap\n"),
		},
	}}
	cipher, err := NewSecretCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}

	var remoteConfig *configv1.Config
	remoteConfig, err = loadRemoteConfigSourceConfigsWithDecoder([]kratosconfig.Source{source}, cipher.Decoder())
	if err != nil {
		t.Fatal(err)
	}
	if remoteConfig == nil {
		t.Fatal("expected remote config")
	}
	if remoteConfig.GetType() != "etcd" {
		t.Fatalf("unexpected remote config type: %q", remoteConfig.GetType())
	}
	if remoteConfig.GetEtcd().GetKey() != "kratos.bootstrap" {
		t.Fatalf("unexpected remote config key: %q", remoteConfig.GetEtcd().GetKey())
	}
	if source.watchCalls != 0 {
		t.Fatalf("temporary config started %d watcher(s)", source.watchCalls)
	}
}

// TestConfigProviderStartsWatchers 验证正式配置加载仍会启动 watcher。
func TestConfigProviderStartsWatchers(t *testing.T) {
	previousLogger := kratoslog.Default()
	kratoslog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer kratoslog.SetDefault(previousLogger)

	source := &bootstrapTestSource{keyValues: []*kratosconfig.KeyValue{
		{
			Key:    "server.yaml",
			Format: "yaml",
			Value:  []byte("server:\n  http:\n    addr: :7001\n"),
		},
	}}
	cipher, err := NewSecretCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	provider, err := newConfigProviderWithDecoder([]kratosconfig.Source{source}, nil, cipher.Decoder())
	if err != nil {
		t.Fatal(err)
	}
	if err = provider.Load(); err != nil {
		t.Fatal(err)
	}
	if source.watchCalls != 1 {
		t.Fatalf("formal config started %d watcher(s), want 1", source.watchCalls)
	}
	changed := make(chan struct{})
	if err = provider.Watch("server.http.addr", func(string, kratosconfig.Value) {
		close(changed)
	}); err != nil {
		t.Fatal(err)
	}
	source.watcher.updates <- []*kratosconfig.KeyValue{
		{
			Key:    "server.yaml",
			Format: "yaml",
			Value:  []byte("server:\n  http:\n    addr: :7002\n"),
		},
	}
	select {
	case <-changed:
	case <-time.After(time.Second):
		t.Fatal("formal config watcher did not observe the update")
	}
	if err = provider.Close(); err != nil {
		t.Fatal(err)
	}
}
