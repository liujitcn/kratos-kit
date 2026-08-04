package baidu

import (
	"fmt"
	"net/http"

	baiduTranslator "github.com/liujitcn/go-utils/translator/baidu"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewTranslator 根据配置创建百度翻译适配器。
func NewTranslator(cfg *configv1.Translator, httpClient *http.Client) (*baiduTranslator.Translator, error) {
	providerConfig := cfg.GetBaidu()
	if providerConfig == nil {
		return nil, fmt.Errorf("translator: baidu config is nil")
	}
	return baiduTranslator.NewTranslator(
		providerConfig.GetAppId(),
		providerConfig.GetSecretKey(),
		baiduTranslator.WithHTTPClient(httpClient),
	), nil
}
