package polaris

import (
	"github.com/go-kratos/kratos/v3/config"

	polarisApi "github.com/polarismesh/polaris-go"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	bConfig "github.com/liujitcn/kratos-kit/config"
)

func init() {
	bConfig.MustRegisterFactory(bConfig.TypePolaris, NewConfigSource)
}

// NewConfigSource 创建一个远程配置源 - Polaris
func NewConfigSource(cfg *configv1.Config) (config.Source, error) {
	if cfg == nil || cfg.Polaris == nil {
		return nil, nil
	}

	configApi, err := polarisApi.NewConfigAPI()
	if err != nil {
		return nil, err
	}

	var opts []Option
	opts = append(opts, WithNamespace(cfg.Polaris.Namespace))
	opts = append(opts, WithFileGroup(cfg.Polaris.FileGroup))
	opts = append(opts, WithFileName(cfg.Polaris.FileName))

	src, err := New(configApi, opts...)
	if err != nil {
		return nil, err
	}

	return src, nil
}
