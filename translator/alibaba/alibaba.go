package alibaba

import (
	"fmt"

	"github.com/liujitcn/go-utils/translator/alibaba"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewTranslator 根据配置创建阿里云翻译适配器。
func NewTranslator(cfg *configv1.Translator) (*alibaba.Translator, error) {
	providerConfig := cfg.GetAlibaba()
	if providerConfig == nil {
		return nil, fmt.Errorf("translator: alibaba config is nil")
	}
	return alibaba.NewTranslator(
		providerConfig.GetAccessKeyId(),
		providerConfig.GetAccessKeySecret(),
		alibaba.WithRegionID(providerConfig.GetRegionId()),
	)
}
