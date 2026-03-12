# kratos-kit/swagger-ui

`swagger-ui` 提供可嵌入到 Go HTTP 服务中的 Swagger UI 页面，适用于标准 `net/http` 与 Kratos HTTP Server。

模块路径：`github.com/liujitcn/kratos-kit/swagger-ui`

## 安装

```bash
go get github.com/liujitcn/kratos-kit/swagger-ui@latest
```

## 对外 API

### 1. `New`

最简方式，直接指定标题、OpenAPI 文档 URL 和挂载路径：

```go
handler := swaggerUI.New("Petstore", "https://petstore3.swagger.io/api/v3/openapi.yaml", "/docs/")
```

### 2. `NewWithOption`

通过 `Option` 构造 handler，适合需要定制 UI 行为的场景：

```go
handler := swaggerUI.NewWithOption(
	swaggerUI.WithTitle("Petstore"),
	swaggerUI.WithRemoteFileURL("https://petstore3.swagger.io/api/v3/openapi.yaml"),
	swaggerUI.WithBasePath("/docs/"),
)
```

### 3. `RegisterSwaggerUIServer`

把 Swagger UI 注册到实现了 `HandlePrefix` 的服务对象（例如 Kratos HTTP Server）：

```go
swaggerUI.RegisterSwaggerUIServer(srv, "Petstore", "https://petstore3.swagger.io/api/v3/openapi.yaml", "/docs/")
```

### 4. `RegisterSwaggerUIServerWithOption`

带 `Option` 的注册方式，支持本地文件与内存数据自动注册文档路由：

```go
swaggerUI.RegisterSwaggerUIServerWithOption(
	srv,
	swaggerUI.WithTitle("Petstore"),
	swaggerUI.WithBasePath("/docs/"),
	swaggerUI.WithLocalFile("./openapi.yaml"),
)
```

## Option 说明

- `WithTitle(title string)`：页面标题。
- `WithBasePath(path string)`：Swagger UI 挂载路径，内部会自动规范化为以 `/` 结尾。
- `WithRemoteFileURL(url string)`：远程 OpenAPI 文档地址（JSON/YAML 均可）。
- `WithLocalFile(filePath string)`：本地 OpenAPI 文件路径。
- `WithMemoryData(content []byte, ext string)`：内存中的 OpenAPI 内容与扩展名（如 `json`、`yaml`）。
- `WithShowTopBar(show bool)`：是否显示顶部导航栏。
- `WithHideCurl(hide bool)`：是否隐藏 curl 代码片段。
- `WithJsonEditor(enable bool)`：是否开启 JSON Editor（实验能力）。
- `WithPreAuthorizeApiKey(keys map[string]string)`：预置 API Key。
- `WithSettingsUI(settings map[string]string)`：覆盖 SwaggerUIBundle 配置项。

## 使用示例

### 标准 net/http

```go
package main

import (
	"net/http"

	swaggerUI "github.com/liujitcn/kratos-kit/swagger-ui"
)

func main() {
	h := swaggerUI.NewWithOption(
		swaggerUI.WithTitle("Petstore"),
		swaggerUI.WithRemoteFileURL("https://petstore3.swagger.io/api/v3/openapi.yaml"),
		swaggerUI.WithBasePath("/docs/"),
	)

	http.Handle("/docs/", h)
	_ = http.ListenAndServe(":8080", http.DefaultServeMux)
}
```

### Kratos HTTP Server

```go
package server

import (
	rest "github.com/go-kratos/kratos/v2/transport/http"
	swaggerUI "github.com/liujitcn/kratos-kit/swagger-ui"
)

func RegisterDocs(srv *rest.Server) {
	swaggerUI.RegisterSwaggerUIServerWithOption(
		srv,
		swaggerUI.WithTitle("My API"),
		swaggerUI.WithBasePath("/docs/"),
		swaggerUI.WithRemoteFileURL("https://petstore3.swagger.io/api/v3/openapi.yaml"),
	)
}
```

### Kratos + 本地文件

```go
swaggerUI.RegisterSwaggerUIServerWithOption(
	srv,
	swaggerUI.WithTitle("My API"),
	swaggerUI.WithBasePath("/docs/"),
	swaggerUI.WithLocalFile("./openapi.yaml"),
)
```

上述配置会自动注册文档路由（例如 `/docs/openapi.yaml`），并把 Swagger UI 指向该路由。

### Kratos + 内存数据

```go
swaggerUI.RegisterSwaggerUIServerWithOption(
	srv,
	swaggerUI.WithTitle("My API"),
	swaggerUI.WithBasePath("/docs/"),
	swaggerUI.WithMemoryData(openapiBytes, "json"),
)
```

## 注意事项

- 建议 `basePath` 使用独立前缀（如 `/docs/`），避免与业务路由冲突。
- 使用 `WithLocalFile` / `WithMemoryData` 时，建议优先使用 `RegisterSwaggerUIServerWithOption`，以便自动注册 OpenAPI 文档路由。
- `WithSettingsUI` 的值是原样注入到 SwaggerUIBundle 的 JavaScript 配置，请确保内容合法。
