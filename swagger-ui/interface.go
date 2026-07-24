package swaggerUI

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/liujitcn/kratos-kit/swagger-ui/internal/swagger"
)

type httpServerInterface interface {
	HandlePrefix(prefix string, h http.Handler)
	Handle(path string, h http.Handler)
	HandleFunc(path string, h http.HandlerFunc)
}

type openAPIHTTPServerInterface interface {
	Handle(path string, h http.Handler)
}

func RegisterSwaggerUIServer[T httpServerInterface](srv T, title, swaggerJSONPath string, basePath string) {
	swaggerHandler := newHandler(title, swaggerJSONPath, basePath)
	srv.HandlePrefix(swaggerHandler.BasePath, swaggerHandler)
}

func RegisterSwaggerUIServerWithOption[T httpServerInterface](srv T, handlerOpts ...HandlerOption) {
	opts := swagger.NewConfig()

	for _, o := range handlerOpts {
		o(opts)
	}

	if opts.LocalOpenApiFile != "" {
		registerOpenApiLocalFileRouter(srv, opts)
	} else if len(opts.OpenApiData) != 0 {
		registerOpenApiMemoryDataRouter(srv, opts)
	}

	swaggerHandler := newHandlerWithConfig(opts)

	srv.HandlePrefix(swaggerHandler.BasePath, swaggerHandler)
}

// RegisterOpenAPIServerWithOption 注册不包含 Swagger UI 静态页面的原始 OpenAPI 文档接口。
func RegisterOpenAPIServerWithOption[T openAPIHTTPServerInterface](srv T, handlerOpts ...HandlerOption) {
	opts := swagger.NewConfig()

	for _, o := range handlerOpts {
		o(opts)
	}
	if opts.OpenAPIPath == "" {
		panic("openapi path is required")
	}
	if !strings.HasPrefix(opts.OpenAPIPath, "/") {
		opts.OpenAPIPath = "/" + opts.OpenAPIPath
	}

	_openAPIHandler, err := newOpenAPIHandlerWithConfig(opts)
	if err != nil {
		panic(err)
	}
	srv.Handle(opts.OpenAPIPath, _openAPIHandler)
}

// var _openJsonFileHandler = &openApiFileHandler{}

func registerOpenApiLocalFileRouter[T httpServerInterface](srv T, cfg *swagger.Config) {
	var _openJsonFileHandler = &openApiFileHandler{}
	err := _openJsonFileHandler.LoadFile(cfg.LocalOpenApiFile)
	if err == nil {
		pattern := strings.TrimRight(cfg.BasePath, "/") + "/openapi" + path.Ext(cfg.LocalOpenApiFile)
		cfg.SwaggerJsonUrl = pattern
		srv.Handle(pattern, _openJsonFileHandler)
	} else {
		fmt.Println("load openapi file failed: ", err)
	}
}

func registerOpenApiMemoryDataRouter[T httpServerInterface](srv T, cfg *swagger.Config) {
	var _openJsonFileHandler = &openApiFileHandler{}
	_openJsonFileHandler.Content = cfg.OpenApiData
	pattern := strings.TrimRight(cfg.BasePath, "/") + "/openapi." + cfg.OpenApiDataType
	cfg.SwaggerJsonUrl = pattern
	srv.Handle(pattern, _openJsonFileHandler)
	cfg.OpenApiData = nil
}
