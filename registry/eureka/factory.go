package eureka

import (
	"github.com/go-kratos/kratos/v3/registry"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Eureka, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Eureka, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Eureka
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Eureka == nil {
		return nil, nil
	}

	var opts []Option
	if c.Eureka.HeartbeatInterval != nil {
		opts = append(opts, WithHeartbeat(c.Eureka.HeartbeatInterval.AsDuration()))
	}
	if c.Eureka.RefreshInterval != nil {
		opts = append(opts, WithRefresh(c.Eureka.RefreshInterval.AsDuration()))
	}
	if c.Eureka.Path != "" {
		opts = append(opts, WithEurekaPath(c.Eureka.Path))
	}

	var err error
	var reg *Registry
	if reg, err = New(c.Eureka.Endpoints, opts...); err != nil {
		return nil, err
	}

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
