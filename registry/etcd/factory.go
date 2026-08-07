package etcd

import (
	"github.com/go-kratos/kratos/v3/registry"

	etcdClient "go.etcd.io/etcd/client/v3"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Etcd, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Etcd, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Etcd
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Etcd == nil {
		return nil, nil
	}

	cfg := etcdClient.Config{
		Endpoints: c.Etcd.Endpoints,
		Username:  c.Etcd.Username,
		Password:  c.Etcd.Password,
	}

	var err error
	var cli *etcdClient.Client
	if cli, err = etcdClient.New(cfg); err != nil {
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
