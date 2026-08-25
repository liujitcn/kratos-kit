package polaris

import (
	"fmt"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/registry"

	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Polaris, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Polaris, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Polaris
func NewRegistry(c *configv1.Registry) (*Registry, error) {
	if c == nil || c.Polaris == nil {
		return nil, nil
	}

	var err error

	var consumer api.ConsumerAPI
	if consumer, err = api.NewConsumerAPI(); err != nil {
		return nil, fmt.Errorf("fail to create consumerAPI: %w", err)
	}

	var provider api.ProviderAPI
	provider = api.NewProviderAPIByContext(consumer.SDKContext())

	log.Info(fmt.Sprintf("start to register instances, count %d", c.Polaris.InstanceCount))

	var resp *model.InstanceRegisterResponse
	for i := 0; i < (int)(c.Polaris.InstanceCount); i++ {
		registerRequest := &api.InstanceRegisterRequest{}
		registerRequest.Service = c.Polaris.Service
		registerRequest.Namespace = c.Polaris.Namespace
		registerRequest.Host = c.Polaris.Address
		registerRequest.Port = (int)(c.Polaris.Port) + i
		registerRequest.ServiceToken = c.Polaris.Token
		registerRequest.SetHealthy(true)
		if resp, err = provider.RegisterInstance(registerRequest); err != nil {
			return nil, fmt.Errorf("fail to register instance %d: %w", i, err)
		}

		log.Info(fmt.Sprintf("register instance %d response: instanceId %s", i, resp.InstanceID))
	}

	reg := New(provider, consumer)

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
