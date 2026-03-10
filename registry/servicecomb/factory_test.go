package servicecomb

import (
	"testing"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/stretchr/testify/assert"
)

func TestNewServicecombRegistry(t *testing.T) {
	cfg := conf.Registry{
		Servicecomb: &conf.Registry_Servicecomb{
			Endpoints: []string{"127.0.0.1:30100"},
		},
	}

	reg, err := NewRegistry(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, reg)
}
