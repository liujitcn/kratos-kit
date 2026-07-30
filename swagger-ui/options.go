package swaggerUI

import (
	"net/http"

	"github.com/liujitcn/kratos-kit/swagger-ui/internal/swagger"
)

// DefaultOpenAPIPath 是原始 OpenAPI 文档接口的默认路径。
const DefaultOpenAPIPath = "/api/docs/openapi"

type HandlerOption func(opt *swagger.Config)

// WithTitle Title of an index file.
func WithTitle(title string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.Title = title
	}
}

// WithBasePath Base URL to docs.
func WithBasePath(path string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.BasePath = path
	}
}

// WithShowTopBar Show navigation top bar, hidden by default.
func WithShowTopBar(show bool) HandlerOption {
	return func(opt *swagger.Config) {
		opt.ShowTopBar = show
	}
}

// WithHideCurl Hide curl code snippet
func WithHideCurl(hide bool) HandlerOption {
	return func(opt *swagger.Config) {
		opt.HideCurl = hide
	}
}

// WithJsonEditor Enable visual JSON editor support (experimental can fail with complex schemas).
func WithJsonEditor(enable bool) HandlerOption {
	return func(opt *swagger.Config) {
		opt.JsonEditor = enable
	}
}

// WithPreAuthorizeApiKey Map of security name to key value
func WithPreAuthorizeApiKey(keys map[string]string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.PreAuthorizeApiKey = keys
	}
}

// WithSettingsUI contains keys and plain javascript values of SwaggerUIBundle configuration.
// Overrides default values.
// See https://swagger.io/docs/open-source-tools/swagger-ui/usage/configuration/ for available options.
func WithSettingsUI(settings map[string]string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.SettingsUI = settings
	}
}

func WithLocalFile(filePath string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.LocalOpenApiFile = filePath
	}
}

func WithMemoryData(content []byte, ext string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.OpenApiData = content
		opt.OpenApiDataType = ext
	}
}

// WithOpenAPIPath 设置原始 OpenAPI 文档接口路径，未设置时使用 DefaultOpenAPIPath。
func WithOpenAPIPath(path string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.OpenAPIPath = path
	}
}

// WithOpenAPIAuthorizer 设置原始 OpenAPI 文档接口的访问校验函数。
func WithOpenAPIAuthorizer(authorizer func(*http.Request) bool) HandlerOption {
	return func(opt *swagger.Config) {
		opt.OpenAPIAuthorizer = authorizer
	}
}

// WithRemoteFileURL URL to openapi.json/swagger.json document specification.
func WithRemoteFileURL(url string) HandlerOption {
	return func(opt *swagger.Config) {
		opt.SwaggerJsonUrl = url
	}
}
