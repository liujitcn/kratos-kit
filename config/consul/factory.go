package consul

import (
	"github.com/go-kratos/kratos/v3/config"

	"github.com/hashicorp/consul/api"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypeConsul, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Consul
func NewConfigSource(c *configv1.Config) (config.Source, error) {
	if c == nil || c.Consul == nil {
		return nil, nil
	}

	cfg := api.DefaultConfig()
	cfg.Address = c.Consul.Address
	cfg.Scheme = c.Consul.Scheme

	cli, err := api.NewClient(cfg)
	if err != nil {
		return nil, err
	}

	src, err := New(cli,
		WithPath(getConfigKey(c.Consul.Key, true)),
	)
	if err != nil {
		return nil, err
	}

	return src, nil
}
