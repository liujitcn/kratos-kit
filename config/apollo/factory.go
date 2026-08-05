package apollo

import (
	"github.com/go-kratos/kratos/v3/config"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypeApollo, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Apollo
func NewConfigSource(cfg *configv1.Config) (config.Source, error) {
	if cfg == nil || cfg.Apollo == nil {
		return nil, nil
	}

	source, err := NewSource(
		WithAppID(cfg.Apollo.AppId),
		WithCluster(cfg.Apollo.Cluster),
		WithEndpoint(cfg.Apollo.Endpoint),
		WithNamespace(cfg.Apollo.Namespace),
		WithSecret(cfg.Apollo.Secret),
		WithEnableBackup(),
	)
	if err != nil {
		return nil, err
	}
	return source, nil
}
