package key

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/aws"
	"github.com/liujitcn/kratos-kit/key/azure"
	"github.com/liujitcn/kratos-kit/key/file"
	"github.com/liujitcn/kratos-kit/key/google"
	"github.com/liujitcn/kratos-kit/key/internal"
	"github.com/liujitcn/kratos-kit/key/kubernetes"
	"github.com/liujitcn/kratos-kit/key/vault"
)

var (
	// ErrInvalidConfig 表示密钥配置无效。
	ErrInvalidConfig = errors.New("key: invalid config")
)

// Key 定义按用途派生业务密钥的能力。
type Key interface {
	// Derive 按用途派生 32 字节业务密钥。
	Derive(context.Context, string) ([]byte, error)
}

// NewKey 根据配置创建密钥 Provider，并返回统一的 Key 接口。
// 未配置或无法识别 Provider 类型时使用本地文件 Provider。
func NewKey(ctx context.Context, cfg *configv1.Key) (Key, error) {
	if cfg == nil {
		cfg = defaultConfig("")
	}
	if cfg.GetScope() == "" {
		cfg.Scope = "default"
	}
	if cfg.GetType() == "" {
		cfg.Type = string(Local)
	}

	rootName := cfg.GetRootName()
	if rootName == "" && cfg.GetFile() != nil {
		rootName = cfg.GetFile().GetPath()
	}
	if rootName == "" {
		return nil, fmt.Errorf("%w: root_name is empty", ErrInvalidConfig)
	}

	var provider internal.Provider
	var err error
	switch Type(strings.ToLower(strings.TrimSpace(cfg.GetType()))) {
	default:
		fallthrough
	case Local:
		provider, err = file.NewFromConfig(cfg)
	case Vault:
		provider, err = vault.NewFromConfig(cfg)
	case AWS:
		provider, err = aws.NewFromConfig(ctx, cfg)
	case Google:
		provider, err = google.NewFromConfig(ctx, cfg)
	case Azure:
		provider, err = azure.NewFromConfig(ctx, cfg)
	case Kubernetes:
		provider, err = kubernetes.NewFromConfig(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}
	return internal.NewResolver(provider, rootName, cfg.GetRootVersion(), cfg.GetScope())
}

func defaultConfig(configPath string) *configv1.Key {
	if configPath == "" {
		configPath = "."
	}
	rootPath := filepath.Join(configPath, "root.key")
	return &configv1.Key{
		Type:     string(Local),
		Scope:    "default",
		RootName: rootPath,
		File:     &configv1.Key_File{Path: rootPath},
	}
}
