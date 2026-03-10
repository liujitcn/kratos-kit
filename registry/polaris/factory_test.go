package polaris

import (
	"testing"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/stretchr/testify/assert"
)

func TestNewPolarisRegistry(t *testing.T) {
	cfg := conf.Registry{
		Polaris: &conf.Registry_Polaris{
			Address:       "127.0.0.1",
			Port:          8091,
			InstanceCount: 5,
			Namespace:     "default",
			Service:       "DiscoverEchoServer",
			Token:         "",
		},
	}

	reg, err := NewRegistry(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, reg)
}
