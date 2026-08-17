package translator

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	utilsTranslator "github.com/liujitcn/go-utils/translator"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	alibabaTranslator "github.com/liujitcn/kratos-kit/translator/alibaba"
	baiduTranslator "github.com/liujitcn/kratos-kit/translator/baidu"
	googleTranslator "github.com/liujitcn/kratos-kit/translator/google"
	volcTranslator "github.com/liujitcn/kratos-kit/translator/volc"
)

const defaultTimeout = 8 * time.Second

// NewTranslator 创建配置指定的翻译 Provider，并返回配置或客户端初始化错误。
func NewTranslator(cfg *configv1.Translator) (utilsTranslator.Translator, error) {
	if cfg == nil || !cfg.GetEnabled() {
		return nil, nil
	}

	providerName := strings.ToLower(cfg.GetType())
	if providerName == "" {
		providerName = Google
	}

	timeout := cfg.GetTimeout().AsDuration()
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	httpClient := &http.Client{Timeout: timeout}
	switch providerName {
	case Google:
		return googleTranslator.NewTranslator(cfg, httpClient)
	case Baidu:
		return baiduTranslator.NewTranslator(cfg, httpClient)
	case Alibaba:
		return alibabaTranslator.NewTranslator(cfg)
	case Volc:
		return volcTranslator.NewTranslator(cfg, httpClient)
	default:
		return nil, fmt.Errorf("translator: unsupported provider %q", providerName)
	}
}
