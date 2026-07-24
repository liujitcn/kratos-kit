package swaggerUI

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/liujitcn/kratos-kit/swagger-ui/internal/swagger"
)

const openAPIUnauthorizedResponse = `{"code":401,"reason":"UNAUTHENTICATED","message":"access token do not exists"}`

type openAPIHandler struct {
	content     []byte
	contentType string
	authorizer  func(*http.Request) bool
}

// newOpenAPIHandlerWithConfig 创建只返回 OpenAPI 原文的 HTTP 处理器。
func newOpenAPIHandlerWithConfig(cfg *swagger.Config) (http.Handler, error) {
	content := cfg.OpenApiData
	dataType := cfg.OpenApiDataType
	if len(content) == 0 && cfg.LocalOpenApiFile != "" {
		fileHandler := &openApiFileHandler{}
		err := fileHandler.LoadFile(cfg.LocalOpenApiFile)
		if err != nil {
			return nil, fmt.Errorf("load openapi file: %w", err)
		}
		content = fileHandler.Content
		dataType = strings.TrimPrefix(path.Ext(cfg.LocalOpenApiFile), ".")
	}
	if len(content) == 0 {
		return nil, fmt.Errorf("openapi document data is required")
	}

	return &openAPIHandler{
		content:     content,
		contentType: openAPIContentType(dataType),
		authorizer:  cfg.OpenAPIAuthorizer,
	}, nil
}

// ServeHTTP 校验访问权限后返回 OpenAPI 文档内容。
func (h *openAPIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.authorizer == nil || !h.authorizer(r) {
		writeOpenAPIUnauthorized(w)
		return
	}

	w.Header().Set("Content-Type", h.contentType)
	if _, err := w.Write(h.content); err != nil {
		return
	}
}

// openAPIContentType 根据文档扩展名返回 HTTP Content-Type。
func openAPIContentType(dataType string) string {
	switch strings.TrimPrefix(strings.ToLower(dataType), ".") {
	case "json":
		return "application/json; charset=utf-8"
	case "yaml", "yml":
		return "application/yaml; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// writeOpenAPIUnauthorized 返回统一的未认证响应。
func writeOpenAPIUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	if _, err := w.Write([]byte(openAPIUnauthorizedResponse)); err != nil {
		return
	}
}
