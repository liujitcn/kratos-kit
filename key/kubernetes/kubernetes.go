package kubernetes

import (
	"context"
	"errors"
	"fmt"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
)

// Option 配置 Kubernetes Secret Provider。
type Option func(*options)

type options struct {
	valueKey string
}

// WithValueKey 设置 Secret.Data 中保存密钥值的字段名，默认值为 value。
func WithValueKey(valueKey string) Option {
	return func(o *options) {
		if valueKey != "" {
			o.valueKey = valueKey
		}
	}
}

// Provider 从 Kubernetes Secret 读取密钥。
type Provider struct {
	secrets typedcorev1.SecretInterface
	options options
}

var _ internal.Provider = (*Provider)(nil)

// New 使用指定命名空间的 SecretInterface 创建 Provider。
func New(secrets typedcorev1.SecretInterface, opts ...Option) (*Provider, error) {
	if secrets == nil {
		return nil, errors.New("key/kubernetes: secret interface is nil")
	}
	provider := &Provider{
		secrets: secrets,
		options: options{valueKey: "value"},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&provider.options)
		}
	}
	return provider, nil
}

// NewFromConfig 根据 Key 配置创建 Kubernetes Secret Provider。
// Provider 使用 Pod 的 ServiceAccount 通过 Kubernetes API 读取 Secret。
func NewFromConfig(_ context.Context, cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetKubernetes() == nil {
		return nil, errors.New("key/kubernetes: kubernetes config is nil")
	}
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("key/kubernetes: create in-cluster config: %w", err)
	}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("key/kubernetes: create client: %w", err)
	}
	options := make([]Option, 0, 1)
	if valueKey := cfg.GetKubernetes().GetValueKey(); valueKey != "" {
		options = append(options, WithValueKey(valueKey))
	}
	return NewFromClient(client, cfg.GetKubernetes().GetNamespace(), options...)
}

// NewFromClient 使用 Kubernetes Clientset 和命名空间创建 Provider。
func NewFromClient(client kubernetes.Interface, namespace string, opts ...Option) (*Provider, error) {
	if client == nil {
		return nil, errors.New("key/kubernetes: client is nil")
	}
	if namespace == "" {
		return nil, errors.New("key/kubernetes: namespace is empty")
	}
	return New(client.CoreV1().Secrets(namespace), opts...)
}

// Get 读取指定 Kubernetes Secret.Data 字段中的密钥值。
// Kubernetes Secret 没有原生版本，返回的 Version 使用资源版本号。
func (p *Provider) Get(ctx context.Context, name, requestedVersion string) (internal.Secret, error) {
	if p == nil || p.secrets == nil {
		return internal.Secret{}, errors.New("key/kubernetes: provider is nil")
	}
	secret, err := p.secrets.Get(ctx, name, v1.GetOptions{})
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/kubernetes: get %q: %w", name, err)
	}
	if secret == nil {
		return internal.Secret{}, fmt.Errorf("key/kubernetes: %w: %s", internal.ErrSecretNotFound, name)
	}
	value, ok := secret.Data[p.options.valueKey]
	if !ok || len(value) == 0 {
		return internal.Secret{}, fmt.Errorf("key/kubernetes: %w: %s/%s", internal.ErrSecretNotFound, name, p.options.valueKey)
	}
	version := secret.ResourceVersion
	if version == "" {
		version = "unversioned"
	}
	if requestedVersion != "" && requestedVersion != version {
		return internal.Secret{}, fmt.Errorf("key/kubernetes: requested version %q does not match resource version %q", requestedVersion, version)
	}
	return internal.Secret{Version: version, Value: append([]byte(nil), value...)}, nil
}
