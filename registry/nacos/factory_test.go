package nacos

import (
	"testing"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/stretchr/testify/assert"
)

func TestNewNacosRegistry(t *testing.T) {
	cfg := conf.Registry{
		Nacos: &conf.Registry_Nacos{
			Address: "127.0.0.1",
			Port:    8848,
		},
	}

	reg, err := NewRegistry(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, reg)
}
