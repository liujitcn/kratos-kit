package google

import (
	"context"
	"errors"
	"fmt"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key/internal"
	"google.golang.org/api/option"
)

type clientAPI interface {
	AccessSecretVersion(context.Context, *secretmanagerpb.AccessSecretVersionRequest, ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
}

// Provider 从 Google Secret Manager 读取密钥。
type Provider struct {
	client  clientAPI
	close   func() error
	project string
}

var _ internal.Provider = (*Provider)(nil)

// New 创建 Google Secret Manager Provider。
func New(ctx context.Context, opts ...option.ClientOption) (*Provider, error) {
	return NewWithProject(ctx, "", opts...)
}

// NewWithProject 创建支持使用短 Secret 名称的 Google Secret Manager Provider。
func NewWithProject(ctx context.Context, project string, opts ...option.ClientOption) (*Provider, error) {
	client, err := secretmanager.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("key/google: create client: %w", err)
	}
	return &Provider{client: client, close: client.Close, project: project}, nil
}

// NewFromConfig 根据 Key 配置创建 Google Secret Manager Provider。
// Google SDK 使用 Application Default Credentials 或工作负载身份。
func NewFromConfig(ctx context.Context, cfg *configv1.Key) (internal.Provider, error) {
	if cfg == nil || cfg.GetGoogle() == nil {
		return nil, errors.New("key/google: google config is nil")
	}
	return NewWithProject(ctx, cfg.GetGoogle().GetProject())
}

func newProvider(client clientAPI) *Provider {
	return &Provider{client: client}
}

// Get 读取指定 Google Secret Manager 版本的密钥值。
// Name 可以是 projects/{project}/secrets/{secret}，Version 为空时读取 latest。
func (p *Provider) Get(ctx context.Context, name, requestedVersion string) (internal.Secret, error) {
	if p == nil || p.client == nil {
		return internal.Secret{}, errors.New("key/google: provider is nil")
	}
	versionedName, err := versionNameWithProject(name, requestedVersion, p.project)
	if err != nil {
		return internal.Secret{}, err
	}
	output, err := p.client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{Name: versionedName})
	if err != nil {
		return internal.Secret{}, fmt.Errorf("key/google: get %q: %w", name, err)
	}
	if output == nil || output.Payload == nil || len(output.Payload.Data) == 0 {
		return internal.Secret{}, fmt.Errorf("key/google: %w: %s", internal.ErrSecretNotFound, name)
	}
	version := requestedVersion
	if output.Name != "" {
		version = versionFromName(output.Name)
	}
	return internal.Secret{Version: version, Value: append([]byte(nil), output.Payload.Data...)}, nil
}

// Close 关闭 Google Secret Manager 客户端。
func (p *Provider) Close() error {
	if p == nil || p.close == nil {
		return nil
	}
	return p.close()
}

func versionName(name, version string) (string, error) {
	return versionNameWithProject(name, version, "")
}

func versionNameWithProject(name, version, project string) (string, error) {
	if strings.Contains(name, "/versions/") {
		if version != "" {
			return "", fmt.Errorf("key/google: version is specified twice for %q", name)
		}
		return name, nil
	}
	if version == "" {
		version = "latest"
	}
	secretName := name
	if !strings.Contains(secretName, "/secrets/") && project != "" {
		secretName = "projects/" + project + "/secrets/" + strings.Trim(name, "/")
	}
	if !strings.Contains(secretName, "/secrets/") {
		return "", fmt.Errorf("key/google: invalid secret name %q", name)
	}
	return strings.TrimRight(secretName, "/") + "/versions/" + version, nil
}

func versionFromName(name string) string {
	parts := strings.Split(strings.TrimRight(name, "/"), "/versions/")
	if len(parts) == 2 && parts[1] != "" {
		return parts[1]
	}
	return "unversioned"
}
