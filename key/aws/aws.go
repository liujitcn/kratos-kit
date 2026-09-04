package aws

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
)

// Option 配置 AWS Secrets Manager Provider。
type Option func(*options)

type options struct {
	versionStage string
}

// WithVersionStage 指定 AWS Secret 的版本阶段，例如 AWSCURRENT。
func WithVersionStage(stage string) Option {
	return func(o *options) {
		o.versionStage = stage
	}
}

type clientAPI interface {
	GetSecretValue(context.Context, *secretsmanager.GetSecretValueInput, ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// Provider 从 AWS Secrets Manager 读取密钥。
type Provider struct {
	client  clientAPI
	options options
}

var _ internal.Provider = (*Provider)(nil)

// New 创建 AWS Secrets Manager Provider。
func New(client *secretsmanager.Client, opts ...Option) (*Provider, error) {
	if client == nil {
		return nil, errors.New("key/aws: client is nil")
	}
	return newProvider(client, opts...), nil
}

// NewFromConfig 根据 Key 配置创建 AWS Secrets Manager Provider。
// AWS SDK 使用默认 credential chain，生产环境建议使用 IAM Role 或工作负载身份。
func NewFromConfig(ctx context.Context, cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetAws() == nil {
		return nil, errors.New("key/aws: aws config is nil")
	}
	options := make([]func(*config.LoadOptions) error, 0, 1)
	if region := cfg.GetAws().GetRegion(); region != "" {
		options = append(options, config.WithRegion(region))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("key/aws: load config: %w", err)
	}
	providerOptions := make([]Option, 0, 1)
	if stage := cfg.GetAws().GetVersionStage(); stage != "" {
		providerOptions = append(providerOptions, WithVersionStage(stage))
	}
	return New(secretsmanager.NewFromConfig(awsCfg), providerOptions...)
}

func newProvider(client clientAPI, opts ...Option) *Provider {
	provider := &Provider{client: client}
	for _, opt := range opts {
		if opt != nil {
			opt(&provider.options)
		}
	}
	return provider
}

// Get 读取 AWS Secrets Manager 中的密钥值。
func (p *Provider) Get(ctx context.Context, name string) (internal.Secret, error) {
	if p == nil || p.client == nil {
		return internal.Secret{}, errors.New("key/aws: provider is nil")
	}
	input := &secretsmanager.GetSecretValueInput{SecretId: &name}
	if p.options.versionStage != "" {
		input.VersionStage = &p.options.versionStage
	}
	output, err := p.client.GetSecretValue(ctx, input)
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/aws: get %q: %w", name, err)
	}
	if output == nil {
		return internal.Secret{}, fmt.Errorf("key/aws: %w: %s", internal.ErrSecretNotFound, name)
	}
	value := output.SecretBinary
	if output.SecretString != nil {
		value = []byte(*output.SecretString)
	}
	if len(value) == 0 {
		return internal.Secret{}, fmt.Errorf("key/aws: %w: %s", internal.ErrSecretNotFound, name)
	}
	return internal.Secret{Value: append([]byte(nil), value...)}, nil
}
