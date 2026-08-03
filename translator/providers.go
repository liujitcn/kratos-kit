package translator

import (
	"fmt"
	"net/http"

	alibabaTranslator "github.com/liujitcn/go-utils/translator/alibaba"
	baiduTranslator "github.com/liujitcn/go-utils/translator/baidu"
	googleTranslator "github.com/liujitcn/go-utils/translator/google"
	volcTranslator "github.com/liujitcn/go-utils/translator/volc"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

func init() {
	MustRegisterProvider(Google, newGoogleTranslator)
	MustRegisterProvider(Baidu, newBaiduTranslator)
	MustRegisterProvider(Alibaba, newAlibabaTranslator)
	MustRegisterProvider(Volc, newVolcTranslator)
}

func newGoogleTranslator(cfg *configv1.Translator, httpClient *http.Client) (Translator, error) {
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

func newBaiduTranslator(cfg *configv1.Translator, httpClient *http.Client) (Translator, error) {
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

func newAlibabaTranslator(cfg *configv1.Translator, _ *http.Client) (Translator, error) {
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

func newVolcTranslator(cfg *configv1.Translator, httpClient *http.Client) (Translator, error) {
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
