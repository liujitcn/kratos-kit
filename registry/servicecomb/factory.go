package servicecomb

import (
	"github.com/go-kratos/kratos/v3/registry"

	"github.com/go-chassis/sc-client"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Servicecomb, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Servicecomb, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Servicecomb
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Servicecomb == nil {
		return nil, nil
	}

	cfg := sc.Options{
		Endpoints: c.Servicecomb.Endpoints,
	}

	var cli *sc.Client
	var err error
	if cli, err = sc.NewClient(cfg); err != nil {
		return nil, err
	}

	reg := New(cli)

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
