package consul

import (
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/log"

	consulApi "github.com/hashicorp/consul/api"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypeConsul, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Consul
func NewConfigSource(c *conf.Config) (config.Source, error) {
	if c == nil || c.Consul == nil {
		return nil, nil
	}

	cfg := consulApi.DefaultConfig()
	cfg.Address = c.Consul.Address
	cfg.Scheme = c.Consul.Scheme

	cli, err := consulApi.NewClient(cfg)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	src, err := New(cli,
		WithPath(getConfigKey(c.Consul.Key, true)),
	)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	return src, nil
}
