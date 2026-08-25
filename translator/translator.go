package translator

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/liujitcn/go-utils/translator"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/translator/alibaba"
	"github.com/liujitcn/kratos-kit/translator/baidu"
	"github.com/liujitcn/kratos-kit/translator/google"
	"github.com/liujitcn/kratos-kit/translator/volc"
)

const defaultTimeout = 8 * time.Second

// NewTranslator 创建配置指定的翻译 Provider，并返回配置或客户端初始化错误。
func NewTranslator(cfg *configv1.Translator) (translator.Translator, error) {
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
		return google.NewTranslator(cfg, httpClient)
	case Baidu:
		return baidu.NewTranslator(cfg, httpClient)
	case Alibaba:
		return alibaba.NewTranslator(cfg)
	case Volc:
		return volc.NewTranslator(cfg, httpClient)
	default:
		return nil, fmt.Errorf("translator: unsupported provider %q", providerName)
	}
}
