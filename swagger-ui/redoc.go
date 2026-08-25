package swaggerUI

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/mvrilo/go-redoc"
)

// RedocConfig 配置 ReDoc 页面和 OpenAPI 文档来源。
type RedocConfig struct {
	Title       string
	Description string
	BasePath    string
	SpecPath    string
	SpecURL     string
	SpecFile    string
	SpecData    []byte
	SpecType    string
	Authorizer  func(*http.Request) bool
}

// RedocOption 配置 ReDoc handler。
type RedocOption func(*RedocConfig)

// RedocHandler 提供 ReDoc 页面及可选的本地 OpenAPI 文档。
type RedocHandler struct {
	basePath     string
	specPath     string
	specData     []byte
	contentType  string
	authorizer   func(*http.Request) bool
	template     *template.Template
	templateData redocTemplateData
}

type redocTemplateData struct {
	Title       string
	Description string
	SpecURL     string
	JavaScript  template.JS
}

// WithRedocTitle 设置 ReDoc 页面标题。
func WithRedocTitle(title string) RedocOption {
	return func(config *RedocConfig) {
		config.Title = title
	}
}

// WithRedocDescription 设置 ReDoc 页面描述。
func WithRedocDescription(description string) RedocOption {
	return func(config *RedocConfig) {
		config.Description = description
	}
}

// WithRedocBasePath 设置 ReDoc 页面挂载路径。
func WithRedocBasePath(basePath string) RedocOption {
	return func(config *RedocConfig) {
		config.BasePath = basePath
	}
}

// WithRedocSpecPath 设置本地或内存 OpenAPI 文档的访问路径。
func WithRedocSpecPath(specPath string) RedocOption {
	return func(config *RedocConfig) {
		config.SpecPath = specPath
	}
}

// WithRedocRemoteFileURL 设置浏览器直接访问的远程 OpenAPI 文档地址。
func WithRedocRemoteFileURL(specURL string) RedocOption {
	return func(config *RedocConfig) {
		config.SpecURL = specURL
	}
}

// WithRedocLocalFile 设置由服务端读取并托管的 OpenAPI 文件。
func WithRedocLocalFile(filePath string) RedocOption {
	return func(config *RedocConfig) {
		config.SpecFile = filePath
	}
}

// WithRedocMemoryData 设置由服务端托管的内存 OpenAPI 文档。
func WithRedocMemoryData(data []byte, dataType string) RedocOption {
	return func(config *RedocConfig) {
		config.SpecData = slices.Clone(data)
		config.SpecType = dataType
	}
}

// WithRedocAuthorizer 设置 ReDoc 页面和本地文档的访问校验函数。
func WithRedocAuthorizer(authorizer func(*http.Request) bool) RedocOption {
	return func(config *RedocConfig) {
		config.Authorizer = authorizer
	}
}

// NewRedocHandler 创建可挂载到 Kratos HTTP Server 的 ReDoc handler。
func NewRedocHandler(options ...RedocOption) (*RedocHandler, error) {
	config := RedocConfig{
		Title:    "API Documentation",
		BasePath: "/docs/",
		SpecType: "json",
	}
	for _, option := range options {
		option(&config)
	}

	basePath := normalizeRedocPath(config.BasePath, true)
	specPath := normalizeRedocPath(config.SpecPath, false)
	if specPath == "" {
		specPath = strings.TrimRight(basePath, "/") + "/openapi.json"
	}

	specURL := config.SpecURL
	specData := slices.Clone(config.SpecData)
	specType := config.SpecType
	dataSources := 0
	if specURL != "" {
		dataSources++
	}
	if config.SpecFile != "" {
		dataSources++
	}
	if len(specData) > 0 {
		dataSources++
	}
	if dataSources != 1 {
		return nil, fmt.Errorf("exactly one ReDoc OpenAPI source is required")
	}

	if config.SpecFile != "" {
		var err error
		specData, err = os.ReadFile(config.SpecFile)
		if err != nil {
			return nil, fmt.Errorf("read ReDoc OpenAPI file: %w", err)
		}
		specType = strings.TrimPrefix(path.Ext(config.SpecFile), ".")
	}
	if specURL == "" {
		specURL = specPath
	}

	pageTemplate, err := template.New("redoc").Parse(redocHTMLTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse ReDoc template: %w", err)
	}

	return &RedocHandler{
		basePath:    basePath,
		specPath:    specPath,
		specData:    specData,
		contentType: openAPIContentType(specType),
		authorizer:  config.Authorizer,
		template:    pageTemplate,
		templateData: redocTemplateData{
			Title:       config.Title,
			Description: config.Description,
			SpecURL:     specURL,
			JavaScript:  template.JS(redoc.JavaScript),
		},
	}, nil
}

// BasePath 返回 ReDoc 页面挂载路径。
func (h *RedocHandler) BasePath() string {
	return h.basePath
}

// SpecPath 返回本地或内存 OpenAPI 文档路径。
func (h *RedocHandler) SpecPath() string {
	return h.specPath
}

// ServeHTTP 返回 ReDoc 页面或由服务端托管的 OpenAPI 文档。
func (h *RedocHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if h.authorizer != nil && !h.authorizer(request) {
		writeOpenAPIUnauthorized(writer)
		return
	}
	if len(h.specData) > 0 && request.URL.Path == h.specPath {
		writer.Header().Set("Content-Type", h.contentType)
		_, _ = writer.Write(h.specData)
		return
	}

	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.template.Execute(writer, h.templateData); err != nil {
		return
	}
}

// RegisterRedocServer 创建并挂载 ReDoc handler。
func RegisterRedocServer[T httpServerInterface](server T, options ...RedocOption) (*RedocHandler, error) {
	handler, err := NewRedocHandler(options...)
	if err != nil {
		return nil, err
	}
	server.HandlePrefix(handler.BasePath(), handler)
	if len(handler.specData) > 0 && !strings.HasPrefix(handler.SpecPath(), handler.BasePath()) {
		server.Handle(handler.SpecPath(), handler)
	}
	return handler, nil
}

// normalizeRedocPath 规范 ReDoc 的绝对挂载路径。
func normalizeRedocPath(value string, trailingSlash bool) string {
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if trailingSlash {
		return strings.TrimRight(value, "/") + "/"
	}
	return strings.TrimRight(value, "/")
}

const redocHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  {{if .Description}}<meta name="description" content="{{.Description}}">{{end}}
</head>
<body>
  <div id="redoc-container"></div>
  <script>{{.JavaScript}}</script>
  <script>Redoc.init({{.SpecURL}}, {}, document.getElementById('redoc-container'));</script>
</body>
</html>`
