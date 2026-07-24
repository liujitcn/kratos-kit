package kubernetes

import (
	"github.com/go-kratos/kratos/v3/config"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypeKubernetes, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Kubernetes
func NewConfigSource(c *configv1.Config) (config.Source, error) {
	if c == nil || c.Kubernetes == nil {
		return nil, nil
	}

	source := NewSource(
		WithNamespace(c.Kubernetes.Namespace),
		WithLabelSelector(c.Kubernetes.LabelSelector),
		WithFieldSelector(c.Kubernetes.FieldSelector),
		WithMaster(c.Kubernetes.Master),
		WithKubeConfig(c.Kubernetes.KubeConfig),
	)
	return source, nil
}
