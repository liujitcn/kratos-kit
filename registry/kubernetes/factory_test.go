package kubernetes

import (
	"testing"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/stretchr/testify/assert"
)

func TestNewKubernetesRegistry(t *testing.T) {
	var cfg conf.Registry
	reg, err := NewRegistry(&cfg)
	assert.Nil(t, err)
	assert.NotNil(t, reg)
}
