package file

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
)

// Provider 从本地文件读取密钥，适用于开发环境或挂载的 Kubernetes Secret。
type Provider struct {
	path string
}

var _ internal.Provider = (*Provider)(nil)

// New 创建固定文件路径的密钥 Provider。
func New(path string) (*Provider, error) {
	if path == "" {
		return nil, errors.New("key/file: path is empty")
	}
	return &Provider{path: path}, nil
}

// NewFromConfig 根据 Key 配置创建本地文件密钥 Provider。
func NewFromConfig(cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetFile() == nil {
		return nil, errors.New("key/file: file config is nil")
	}
	path := cfg.GetFile().GetPath()
	if path == "" {
		path = cfg.GetRootName()
	}
	return New(path)
}

// Get 读取密钥文件内容，并使用文件修改时间作为版本标识。
func (p *Provider) Get(_ context.Context, name, requestedVersion string) (internal.Secret, error) {
	if p == nil || p.path == "" {
		return internal.Secret{}, errors.New("key/file: provider is nil")
	}
	if name != p.path {
		return internal.Secret{}, fmt.Errorf("key/file: reference %q does not match configured path %q", name, p.path)
	}

	value, err := os.ReadFile(p.path)
	if err != nil {
		if os.IsNotExist(err) {
			return internal.Secret{}, fmt.Errorf("key/file: secret not found: %s", p.path)
		}
		return internal.Secret{}, fmt.Errorf("key/file: read %q: %w", p.path, err)
	}
	info, err := os.Stat(p.path)
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/file: stat %q: %w", p.path, err)
	}
	version := info.ModTime().UTC().Format(time.RFC3339Nano)
	if requestedVersion != "" {
		// 文件系统没有原生版本号，调用方指定版本时由配置承担轮换标识。
		version = requestedVersion
	}
	return internal.Secret{Version: version, Value: value}, nil
}
