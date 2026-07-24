package zookeeper

import (
	"github.com/go-kratos/kratos/v3/registry"

	"github.com/go-zookeeper/zk"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.ZooKeeper, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.ZooKeeper, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - ZooKeeper
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Zookeeper == nil {
		return nil, nil
	}

	conn, _, err := zk.Connect(c.Zookeeper.Endpoints, c.Zookeeper.Timeout.AsDuration())
	if err != nil {
		return nil, err
	}

	reg := New(conn)

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
