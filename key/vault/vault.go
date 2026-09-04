package vault

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/vault/api"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
)

// Option 配置 Vault 密钥 Provider。
type Option func(*options)

type options struct {
	valueKey string
}

// WithValueKey 设置 KV Secret 中保存密钥值的字段名，默认值为 value。
func WithValueKey(valueKey string) Option {
	return func(o *options) {
		if valueKey != "" {
			o.valueKey = valueKey
		}
	}
}

type logicalReader interface {
	ReadWithDataWithContext(context.Context, string, map[string][]string) (*api.Secret, error)
}

// Provider 从 Vault KV v1/v2 读取密钥。
type Provider struct {
	logical logicalReader
	options options
}

var _ internal.Provider = (*Provider)(nil)

// New 创建 Vault 密钥 Provider。
func New(client *api.Client, opts ...Option) (*Provider, error) {
	if client == nil {
		return nil, errors.New("key/vault: client is nil")
	}
	return newProvider(client.Logical(), opts...), nil
}

// NewFromConfig 根据 Key 配置创建 Vault 密钥 Provider。
func NewFromConfig(cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetVault() == nil {
		return nil, errors.New("key/vault: vault config is nil")
	}
	vaultConfig := api.DefaultConfig()
	if err := vaultConfig.ReadEnvironment(); err != nil {
		return nil, fmt.Errorf("key/vault: read environment: %w", err)
	}
	if address := cfg.GetVault().GetAddress(); address != "" {
		vaultConfig.Address = address
	}
	client, err := api.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("key/vault: create client: %w", err)
	}
	if namespace := cfg.GetVault().GetNamespace(); namespace != "" {
		client.SetNamespace(namespace)
	}
	if valueKey := cfg.GetVault().GetValueKey(); valueKey != "" {
		return New(client, WithValueKey(valueKey))
	}
	return New(client)
}

func newProvider(logical logicalReader, opts ...Option) *Provider {
	provider := &Provider{
		logical: logical,
		options: options{valueKey: "value"},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&provider.options)
		}
	}
	return provider
}

// Get 读取 Vault 路径中的最新密钥值。
func (p *Provider) Get(ctx context.Context, name string) (internal.Secret, error) {
	if p == nil || p.logical == nil {
		return internal.Secret{}, errors.New("key/vault: provider is nil")
	}
	secret, err := p.logical.ReadWithDataWithContext(ctx, name, nil)
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/vault: read %q: %w", name, err)
	}
	if secret == nil || secret.Data == nil {
		return internal.Secret{}, fmt.Errorf("key/vault: %w: %s", internal.ErrSecretNotFound, name)
	}

	value, err := extractValue(secret.Data, p.options.valueKey)
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/vault: read %q: %w", name, err)
	}
	return internal.Secret{Value: value}, nil
}

func extractValue(data map[string]interface{}, valueKey string) ([]byte, error) {
	if inner, ok := data["data"].(map[string]interface{}); ok {
		data = inner
	}
	value, ok := data[valueKey]
	if !ok {
		return nil, fmt.Errorf("%w: field %q is missing", internal.ErrSecretNotFound, valueKey)
	}
	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return append([]byte(nil), typed...), nil
	default:
		return nil, fmt.Errorf("field %q has unsupported type %T", valueKey, value)
	}
}
