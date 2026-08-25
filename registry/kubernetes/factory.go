package kubernetes

import (
	"path/filepath"

	"github.com/go-kratos/kratos/v3/registry"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	baseRegistry "github.com/liujitcn/kratos-kit/registry"
)

func init() {
	_ = baseRegistry.RegisterDiscoveryFactory(baseRegistry.Kubernetes, NewDiscovery)
	_ = baseRegistry.RegisterRegistrarFactory(baseRegistry.Kubernetes, NewRegistrar)
}

// NewRegistry 创建一个注册发现客户端 - Kubernetes
func NewRegistry(cfg *configv1.Registry) (*Registry, error) {
	if cfg == nil || cfg.Kubernetes == nil {
		return nil, nil
	}

	restConfig, err := rest.InClusterConfig()
	if err != nil {
		home := homedir.HomeDir()
		kubeConfig := filepath.Join(home, ".kube", "config")
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			return nil, err
		}
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	var namespace string
	reg := New(clientSet, namespace)

	return reg, nil
}

func NewDiscovery(c *configv1.Registry) (registry.Discovery, error) {
	return NewRegistry(c)
}

func NewRegistrar(c *configv1.Registry) (registry.Registrar, error) {
	return NewRegistry(c)
}
