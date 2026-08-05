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

// RegisterSwaggerUIServer 创建并注册 Swagger UI 处理器，并返回初始化错误。
func RegisterSwaggerUIServer[T httpServerInterface](srv T, title, swaggerJSONPath string, basePath string) error {
	swaggerHandler, err := newHandler(title, swaggerJSONPath, basePath)
	if err != nil {
		return err
	}
	srv.HandlePrefix(swaggerHandler.BasePath, swaggerHandler)
	return nil
}

// RegisterSwaggerUIServerWithOption 根据选项创建并注册 Swagger UI 处理器，并返回初始化错误。
func RegisterSwaggerUIServerWithOption[T httpServerInterface](srv T, handlerOpts ...HandlerOption) error {
	opts := swagger.NewConfig()

	for _, o := range handlerOpts {
		o(opts)
	}

	if opts.LocalOpenApiFile != "" {
		if err := registerOpenApiLocalFileRouter(srv, opts); err != nil {
			return err
		}
	} else if len(opts.OpenApiData) != 0 {
		registerOpenApiMemoryDataRouter(srv, opts)
	}

	swaggerHandler, err := newHandlerWithConfig(opts)
	if err != nil {
		return err
	}

	srv.HandlePrefix(swaggerHandler.BasePath, swaggerHandler)
	return nil
}

// RegisterOpenAPIServerWithOption 注册不包含 Swagger UI 静态页面的原始 OpenAPI 文档接口。
func RegisterOpenAPIServerWithOption[T openAPIHTTPServerInterface](srv T, handlerOpts ...HandlerOption) error {
	opts := swagger.NewConfig()

	for _, o := range handlerOpts {
		o(opts)
	}
	if opts.OpenAPIPath == "" {
		opts.OpenAPIPath = DefaultOpenAPIPath
	}
	if !strings.HasPrefix(opts.OpenAPIPath, "/") {
		opts.OpenAPIPath = "/" + opts.OpenAPIPath
	}

	_openAPIHandler, err := newOpenAPIHandlerWithConfig(opts)
	if err != nil {
		return err
	}
	srv.Handle(opts.OpenAPIPath, _openAPIHandler)
	return nil
}

// var _openJsonFileHandler = &openApiFileHandler{}

func registerOpenApiLocalFileRouter[T httpServerInterface](srv T, cfg *swagger.Config) error {
	var _openJsonFileHandler = &openApiFileHandler{}
	err := _openJsonFileHandler.LoadFile(cfg.LocalOpenApiFile)
	if err == nil {
		pattern := strings.TrimRight(cfg.BasePath, "/") + "/openapi" + path.Ext(cfg.LocalOpenApiFile)
		cfg.SwaggerJsonUrl = pattern
		srv.Handle(pattern, _openJsonFileHandler)
		return nil
	}
	return fmt.Errorf("load openapi file: %w", err)
}

func registerOpenApiMemoryDataRouter[T httpServerInterface](srv T, cfg *swagger.Config) {
	var _openJsonFileHandler = &openApiFileHandler{}
	_openJsonFileHandler.Content = cfg.OpenApiData
	pattern := strings.TrimRight(cfg.BasePath, "/") + "/openapi." + cfg.OpenApiDataType
	cfg.SwaggerJsonUrl = pattern
	srv.Handle(pattern, _openJsonFileHandler)
	cfg.OpenApiData = nil
}
