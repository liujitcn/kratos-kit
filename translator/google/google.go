package google

import (
	"net/http"

	googleTranslator "github.com/liujitcn/go-utils/translator/google"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewTranslator 根据配置创建 Google 翻译适配器。
func NewTranslator(cfg *configv1.Translator, httpClient *http.Client) (*googleTranslator.Translator, error) {
	providerConfig := cfg.GetGoogle()
	options := []googleTranslator.Option{googleTranslator.WithHTTPClient(httpClient)}
	if providerConfig == nil {
		return googleTranslator.NewTranslator(options...), nil
	}
	if version := providerConfig.GetVersion(); version != "" {
		options = append(options, googleTranslator.WithVersion(version))
	}
	if apiKey := providerConfig.GetApiKey(); apiKey != "" {
		options = append(options, googleTranslator.WithAPIKey(apiKey))
	}
	if projectID := providerConfig.GetProjectId(); projectID != "" {
		options = append(options, googleTranslator.WithProjectID(projectID))
	}
	if location := providerConfig.GetLocation(); location != "" {
		options = append(options, googleTranslator.WithLocation(location))
	}
	if parent := providerConfig.GetParent(); parent != "" {
		options = append(options, googleTranslator.WithParent(parent))
	}
	return googleTranslator.NewTranslator(options...), nil
}
