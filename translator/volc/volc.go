package volc

import (
	"fmt"
	"net/http"

	volcTranslator "github.com/liujitcn/go-utils/translator/volc"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewTranslator 根据配置创建火山引擎翻译适配器。
func NewTranslator(cfg *configv1.Translator, httpClient *http.Client) (*volcTranslator.Translator, error) {
	providerConfig := cfg.GetVolc()
	if providerConfig == nil {
		return nil, fmt.Errorf("translator: volc config is nil")
	}
	return volcTranslator.NewTranslator(
		providerConfig.GetAccessKey(),
		providerConfig.GetSecretKey(),
		volcTranslator.WithRegion(providerConfig.GetRegion()),
		volcTranslator.WithHTTPClient(httpClient),
	)
}
