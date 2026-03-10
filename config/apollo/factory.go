package apollo

import (
	"github.com/go-kratos/kratos/v2/config"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypeApollo, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Apollo
func NewConfigSource(cfg *conf.Config) (config.Source, error) {
	if cfg == nil || cfg.Apollo == nil {
		return nil, nil
	}

	source := NewSource(
		WithAppID(cfg.Apollo.AppId),
		WithCluster(cfg.Apollo.Cluster),
		WithEndpoint(cfg.Apollo.Endpoint),
		WithNamespace(cfg.Apollo.Namespace),
		WithSecret(cfg.Apollo.Secret),
		WithEnableBackup(),
	)
	return source, nil
}
