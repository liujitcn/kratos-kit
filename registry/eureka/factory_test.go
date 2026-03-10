package eureka

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

func TestNewEurekaRegistry(t *testing.T) {
	cfg := conf.Registry{
		Eureka: &conf.Registry_Eureka{
			Endpoints: []string{"https://127.0.0.1:18761"},
		},
	}

	reg, err := NewRegistry(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, reg)
}
