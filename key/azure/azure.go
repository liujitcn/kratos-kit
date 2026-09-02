package azure

import (
	"context"
	"errors"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
)

type clientAPI interface {
	GetSecret(context.Context, string, string, *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error)
}

// Provider 从 Azure Key Vault Secrets 读取密钥。
type Provider struct {
	client clientAPI
}

var _ internal.Provider = (*Provider)(nil)

// New 创建 Azure Key Vault Secrets Provider。
func New(client *azsecrets.Client) (*Provider, error) {
	if client == nil {
		return nil, errors.New("key/azure: client is nil")
	}
	return newProvider(client), nil
}

// NewFromConfig 根据 Key 配置创建 Azure Key Vault Secrets Provider。
// Azure SDK 使用 Managed Identity 或 DefaultAzureCredential。
func NewFromConfig(ctx context.Context, cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetAzure() == nil {
		return nil, errors.New("key/azure: azure config is nil")
	}
	vaultURL := cfg.GetAzure().GetVaultUrl()
	if vaultURL == "" {
		return nil, errors.New("key/azure: vault url is empty")
	}
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("key/azure: create credential: %w", err)
	}
	client, err := azsecrets.NewClient(vaultURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("key/azure: create client: %w", err)
	}
	return New(client)
}

func newProvider(client clientAPI) *Provider {
	return &Provider{client: client}
}

// Get 读取指定 Azure Key Vault Secret 的密钥值。
// SecretRef.Version 为空时读取最新版本。
func (p *Provider) Get(ctx context.Context, name, requestedVersion string) (internal.Secret, error) {
	if p == nil || p.client == nil {
		return internal.Secret{}, errors.New("key/azure: provider is nil")
	}
	response, err := p.client.GetSecret(ctx, name, requestedVersion, nil)
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/azure: get %q: %w", name, err)
	}
	if response.Value == nil || *response.Value == "" {
		return internal.Secret{}, fmt.Errorf("key/azure: %w: %s", internal.ErrSecretNotFound, name)
	}
	version := requestedVersion
	if response.ID != nil {
		version = response.ID.Version()
	}
	if version == "" {
		version = "unversioned"
	}
	return internal.Secret{Version: version, Value: []byte(*response.Value)}, nil
}
