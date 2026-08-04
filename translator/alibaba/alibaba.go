package alibaba

import (
	"fmt"

	alibabaTranslator "github.com/liujitcn/go-utils/translator/alibaba"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewTranslator 根据配置创建阿里云翻译适配器。
func NewTranslator(cfg *configv1.Translator) (*alibabaTranslator.Translator, error) {
	providerConfig := cfg.GetAlibaba()
	if providerConfig == nil {
		return nil, fmt.Errorf("translator: alibaba config is nil")
	}
	return alibabaTranslator.NewTranslator(
		providerConfig.GetAccessKeyId(),
		providerConfig.GetAccessKeySecret(),
		alibabaTranslator.WithRegionID(providerConfig.GetRegionId()),
	)
}
