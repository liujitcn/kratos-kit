package swaggerUI

import (
	"net/http"

	"github.com/liujitcn/kratos-kit/swagger-ui/internal/swagger"
)

// Handler handles swagger UI request.
type Handler = swagger.Handler

// New 创建 Swagger UI HTTP 处理器，并返回配置或模板初始化错误。
func New(title, swaggerJSONPath string, basePath string) (http.Handler, error) {
	return newHandler(title, swaggerJSONPath, basePath)
}

// NewWithOption 根据选项创建 Swagger UI HTTP 处理器，并返回初始化错误。
func NewWithOption(handlerOpts ...HandlerOption) (http.Handler, error) {
	opts := swagger.NewConfig()

	for _, o := range handlerOpts {
		o(opts)
	}

	return newHandlerWithConfig(opts)
}

// newHandlerWithConfig 根据配置创建 Swagger UI HTTP 处理器。
func newHandlerWithConfig(config *swagger.Config) (*Handler, error) {
	return swagger.NewHandlerWithConfig(config, assetsBase, faviconBase, staticServer)
}

// newHandler 根据基础参数创建 Swagger UI HTTP 处理器。
func newHandler(title, swaggerJSONPath string, basePath string) (*Handler, error) {
	return newHandlerWithConfig(&swagger.Config{
		Title:          title,
		SwaggerJsonUrl: swaggerJSONPath,
		BasePath:       basePath,
	})
}
