package consul

import (
	"github.com/go-kratos/kratos/v3/registry"

	"github.com/hashicorp/consul/api"

	baseRegistry "github.com/liujitcn/kratos-kit/registry"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Consul, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Consul, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Consul
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Consul == nil {
		return nil, nil
	}

	cfg := api.DefaultConfig()
	cfg.Address = c.Consul.GetAddress()
	cfg.Scheme = c.Consul.GetScheme()
	cfg.Token = c.Consul.GetToken()

	var cli *api.Client
	var err error
	if cli, err = api.NewClient(cfg); err != nil {
		return nil, err
	}

	reg := New(cli, WithHealthCheck(c.Consul.GetHealthCheck()))

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
