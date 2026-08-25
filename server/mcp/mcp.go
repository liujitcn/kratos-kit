package mcp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/transport/mcp"
	"github.com/liujitcn/kratos-kit/utils"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// CreateMcpServer 创建独立监听的 MCP 服务端。
func CreateMcpServer(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (*mcp.Server, error) {
	options := []mcp.ServerOption{mcp.WithServerType(mcp.ServerTypeHTTP)}
	var err error
	options, err = initMcpServerConfig(cfg, options, opts...)
	if err != nil {
		return nil, err
	}
	return newConfiguredMcpServer(cfg, options...)
}

// CreateMcpSSEServer 创建独立监听的 Legacy SSE MCP 服务端。
func CreateMcpSSEServer(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (*mcp.Server, error) {
	options, err := initMcpServerConfig(cfg, nil, opts...)
	if err != nil {
		return nil, err
	}
	options = append(options, mcp.WithServerType(mcp.ServerTypeSSE))
	return newConfiguredMcpServer(cfg, options...)
}

// CreateMcpHandler 创建可挂载到已有 HTTP 服务的 MCP 服务端。
func CreateMcpHandler(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (*mcp.Server, error) {
	options, err := initMcpServerConfig(cfg, nil, opts...)
	if err != nil {
		return nil, err
	}
	options = append(options, mcp.WithServerType(mcp.ServerTypeInProcess))
	return newConfiguredMcpServer(cfg, options...)
}

// CreateMcpHTTPHandler 创建标准 http.Handler 形式的 MCP Streamable HTTP 处理器。
func CreateMcpHTTPHandler(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (http.Handler, error) {
	srv, err := CreateMcpHandler(cfg, opts...)
	if err != nil {
		return nil, err
	}
	return srv.HTTPHandler()
}

// CreateMcpSSEHandler 创建标准 http.Handler 形式的 MCP Legacy SSE 处理器。
func CreateMcpSSEHandler(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (http.Handler, error) {
	srv, err := CreateMcpHandler(cfg, opts...)
	if err != nil {
		return nil, err
	}
	return srv.SSEHandler()
}

// WithMcpServerOptions 转发底层官方 MCP SDK 服务端选项。
func WithMcpServerOptions(opts ...func(*mcpsdk.ServerOptions)) mcp.ServerOption {
	serverOptions := &mcpsdk.ServerOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(serverOptions)
		}
	}
	return mcp.WithServerOptions(serverOptions)
}

// initMcpServerConfig 根据 server.mcp 配置构建 MCP 服务端选项。
func initMcpServerConfig(cfg *configv1.Bootstrap, options []mcp.ServerOption, opts ...mcp.ServerOption) ([]mcp.ServerOption, error) {
	serverOptions := make([]mcp.ServerOption, 0, len(options)+len(opts)+12)
	serverOptions = append(serverOptions, options...)
	if cfg == nil || cfg.Server == nil || cfg.Server.Mcp == nil {
		return append(serverOptions, opts...), nil
	}

	mcpCfg := cfg.Server.Mcp
	serverType, err := mcpServerTypeFromConfig(mcpCfg.GetTransport())
	if err != nil {
		return nil, err
	}
	if serverType != "" {
		serverOptions = append(serverOptions, mcp.WithServerType(serverType))
	}
	if mcpCfg.GetNetwork() != "" {
		serverOptions = append(serverOptions, mcp.WithNetwork(mcpCfg.GetNetwork()))
	}
	if mcpCfg.GetAddr() != "" {
		serverOptions = append(serverOptions, mcp.WithListenAddress(mcpCfg.GetAddr()))
	}
	if mcpCfg.GetPath() != "" {
		serverOptions = append(serverOptions, mcp.WithHandlerPath(mcpCfg.GetPath()))
	}
	if mcpCfg.GetShutdownTimeout() != nil {
		serverOptions = append(serverOptions, mcp.WithShutdownTimeout(mcpCfg.GetShutdownTimeout().AsDuration()))
	}
	if mcpCfg.EnableKeepalive != nil {
		serverOptions = append(serverOptions, mcp.WithEnableKeepAlive(mcpCfg.GetEnableKeepalive()))
	}
	if mcpCfg.GetStreamableHttp() != nil {
		serverOptions = append(serverOptions, mcp.WithStreamableHTTPOptions(streamableHTTPOptionsFromConfig(mcpCfg.GetStreamableHttp())))
	}
	if mcpCfg.GetLegacySse() != nil {
		serverOptions = append(serverOptions, mcp.WithSSEOptions(sseOptionsFromConfig(mcpCfg.GetLegacySse())))
	}
	if mcpCfg.GetTls() != nil {
		var tlsCfg *tls.Config
		tlsCfg, err = utils.LoadServerTlsConfig(mcpCfg.GetTls())
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			serverOptions = append(serverOptions, mcp.WithTLSConfig(tlsCfg))
		}
	}

	return append(serverOptions, opts...), nil
}

// newConfiguredMcpServer 创建 MCP 服务端，并注册配置化 HTTP Tool。
func newConfiguredMcpServer(cfg *configv1.Bootstrap, opts ...mcp.ServerOption) (*mcp.Server, error) {
	srv := mcp.NewServer(opts...)
	if cfg == nil || cfg.Server == nil || cfg.Server.Mcp == nil {
		return srv, nil
	}
	err := RegisterMcpHTTPTools(srv, cfg.Server.Mcp.GetHttpTools())
	if err != nil {
		return nil, err
	}
	return srv, nil
}

// RegisterMcpHTTPTools 注册通过 server.mcp.http_tools 配置声明的 HTTP MCP Tool。
func RegisterMcpHTTPTools(srv *mcp.Server, tools []*configv1.Server_Mcp_HttpTool) error {
	if len(tools) == 0 {
		return nil
	}
	if srv == nil || srv.MCPServer() == nil {
		return errors.New("mcp server is nil")
	}
	for _, toolCfg := range tools {
		if toolCfg == nil {
			continue
		}
		toolName := strings.TrimSpace(toolCfg.GetName())
		if toolName == "" {
			return errors.New("mcp http tool name is empty")
		}
		registeredTool := &mcpsdk.Tool{
			Name:        toolName,
			Description: toolCfg.GetDescription(),
			InputSchema: mcpHTTPToolInputSchema(toolCfg),
		}
		currentToolCfg := toolCfg
		srv.MCPServer().AddTool(registeredTool, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return callMcpHTTPTool(ctx, currentToolCfg, req)
		})
	}
	return nil
}

// mcpServerTypeFromConfig 将配置枚举转换为 transport/mcp 的服务类型。
func mcpServerTypeFromConfig(transport configv1.Server_Mcp_Transport) (mcp.ServerType, error) {
	switch transport {
	case configv1.Server_Mcp_UNSPECIFIED:
		return "", nil
	case configv1.Server_Mcp_HTTP:
		return mcp.ServerTypeHTTP, nil
	case configv1.Server_Mcp_SSE:
		return mcp.ServerTypeSSE, nil
	case configv1.Server_Mcp_STDIO:
		return mcp.ServerTypeStdio, nil
	case configv1.Server_Mcp_IN_PROCESS:
		return mcp.ServerTypeInProcess, nil
	default:
		return "", fmt.Errorf("unsupported mcp server transport: %s", transport.String())
	}
}

// streamableHTTPOptionsFromConfig 将配置转换为官方 SDK Streamable HTTP 选项。
func streamableHTTPOptionsFromConfig(cfg *configv1.Server_Mcp_StreamableHttp) *mcpsdk.StreamableHTTPOptions {
	if cfg == nil {
		return nil
	}
	options := &mcpsdk.StreamableHTTPOptions{
		Stateless:                  cfg.GetStateless(),
		JSONResponse:               cfg.GetJsonResponse(),
		DisableLocalhostProtection: cfg.GetDisableLocalhostProtection(),
	}
	if cfg.GetSessionTimeout() != nil {
		options.SessionTimeout = cfg.GetSessionTimeout().AsDuration()
	}
	return options
}

// sseOptionsFromConfig 将配置转换为官方 SDK Legacy SSE 选项。
func sseOptionsFromConfig(cfg *configv1.Server_Mcp_LegacySse) *mcpsdk.SSEOptions {
	if cfg == nil {
		return nil
	}
	return &mcpsdk.SSEOptions{
		DisableLocalhostProtection: cfg.GetDisableLocalhostProtection(),
	}
}

// callMcpHTTPTool 执行一个配置化 HTTP Tool。
func callMcpHTTPTool(ctx context.Context, toolCfg *configv1.Server_Mcp_HttpTool, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
	if toolCfg == nil {
		return nil, errors.New("mcp http tool config is nil")
	}

	var err error
	var args map[string]any
	args, err = mcpHTTPToolArguments(req)
	if err != nil {
		return nil, err
	}

	var httpReq *http.Request
	httpReq, err = buildMcpHTTPToolRequest(ctx, toolCfg, args)
	if err != nil {
		return nil, err
	}

	httpClient := http.DefaultClient
	if toolCfg.GetTimeout() != nil {
		httpClient = &http.Client{Timeout: toolCfg.GetTimeout().AsDuration()}
	}

	var httpResp *http.Response
	httpResp, err = httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	var body []byte
	body, err = io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, err
	}

	result := &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: string(body)}},
	}
	if httpResp.StatusCode < http.StatusOK || httpResp.StatusCode >= http.StatusMultipleChoices {
		result.IsError = true
		if len(body) == 0 {
			result.Content = []mcpsdk.Content{&mcpsdk.TextContent{Text: httpResp.Status}}
		}
		return result, nil
	}

	structuredContent := mcpHTTPToolStructuredContent(httpResp.Header.Get("Content-Type"), body)
	if structuredContent != nil {
		result.StructuredContent = structuredContent
	}
	return result, nil
}

// mcpHTTPToolArguments 解析 MCP Tool 调用参数。
func mcpHTTPToolArguments(req *mcpsdk.CallToolRequest) (map[string]any, error) {
	args := make(map[string]any)
	if req == nil || req.Params == nil || len(req.Params.Arguments) == 0 {
		return args, nil
	}
	err := json.Unmarshal(req.Params.Arguments, &args)
	if err != nil {
		return nil, fmt.Errorf("decode mcp http tool arguments: %w", err)
	}
	return args, nil
}

// buildMcpHTTPToolRequest 根据配置和 Tool 参数构建 HTTP 请求。
func buildMcpHTTPToolRequest(ctx context.Context, toolCfg *configv1.Server_Mcp_HttpTool, args map[string]any) (*http.Request, error) {
	var err error
	var endpoint string
	endpoint, err = mcpHTTPToolEndpoint(toolCfg, args)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	var contentType string
	bodyReader, contentType, err = mcpHTTPToolBody(toolCfg, args)
	if err != nil {
		return nil, err
	}

	method := strings.ToUpper(strings.TrimSpace(toolCfg.GetMethod()))
	if method == "" {
		method = http.MethodGet
	}

	var httpReq *http.Request
	httpReq, err = http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, err
	}
	for key, value := range toolCfg.GetHeaders() {
		httpReq.Header.Set(key, value)
	}
	for _, param := range toolCfg.GetParameters() {
		if param == nil || param.GetLocation() != configv1.Server_Mcp_HTTP_PARAM_LOCATION_HEADER {
			continue
		}
		var value any
		var ok bool
		value, ok, err = mcpHTTPToolParamValue(param, args)
		if err != nil {
			return nil, err
		}
		if ok {
			httpReq.Header.Set(mcpHTTPToolParamTarget(param), fmt.Sprint(value))
		}
	}
	if contentType != "" && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", contentType)
	}
	return httpReq, nil
}

// mcpHTTPToolEndpoint 根据 base_url、url 和参数构建最终请求地址。
func mcpHTTPToolEndpoint(toolCfg *configv1.Server_Mcp_HttpTool, args map[string]any) (string, error) {
	var err error
	rawURL := strings.TrimSpace(toolCfg.GetUrl())
	if rawURL == "" {
		return "", errors.New("mcp http tool url is empty")
	}

	endpoint := rawURL
	if !mcpHTTPToolIsAbsoluteURL(rawURL) {
		baseURL := strings.TrimRight(strings.TrimSpace(toolCfg.GetBaseUrl()), "/")
		if baseURL != "" {
			endpoint = baseURL + "/" + strings.TrimLeft(rawURL, "/")
		}
	}

	for _, param := range toolCfg.GetParameters() {
		if param == nil || param.GetLocation() != configv1.Server_Mcp_HTTP_PARAM_LOCATION_PATH {
			continue
		}
		var value any
		var ok bool
		value, ok, err = mcpHTTPToolParamValue(param, args)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}
		escapedValue := url.PathEscape(fmt.Sprint(value))
		target := mcpHTTPToolParamTarget(param)
		endpoint = strings.ReplaceAll(endpoint, "{"+target+"}", escapedValue)
		endpoint = strings.ReplaceAll(endpoint, ":"+target, escapedValue)
	}

	var parsedURL *url.URL
	parsedURL, err = url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := parsedURL.Query()
	for _, param := range toolCfg.GetParameters() {
		if param == nil || param.GetLocation() != configv1.Server_Mcp_HTTP_PARAM_LOCATION_QUERY {
			continue
		}
		var value any
		var ok bool
		value, ok, err = mcpHTTPToolParamValue(param, args)
		if err != nil {
			return "", err
		}
		if ok {
			query.Set(mcpHTTPToolParamTarget(param), fmt.Sprint(value))
		}
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

// mcpHTTPToolBody 根据配置构建 HTTP 请求 Body。
func mcpHTTPToolBody(toolCfg *configv1.Server_Mcp_HttpTool, args map[string]any) (io.Reader, string, error) {
	bodyParams, err := mcpHTTPToolBodyParams(toolCfg, args)
	if err != nil {
		return nil, "", err
	}

	switch toolCfg.GetBodyMode() {
	case configv1.Server_Mcp_HTTP_BODY_MODE_UNSPECIFIED, configv1.Server_Mcp_HTTP_BODY_MODE_NONE:
		return nil, "", nil
	case configv1.Server_Mcp_HTTP_BODY_MODE_JSON:
		payload := bodyParams
		if len(payload) == 0 {
			payload = args
		}
		var data []byte
		data, err = json.Marshal(payload)
		if err != nil {
			return nil, "", err
		}
		return bytes.NewReader(data), "application/json", nil
	case configv1.Server_Mcp_HTTP_BODY_MODE_FORM:
		values := url.Values{}
		for key, value := range bodyParams {
			values.Set(key, fmt.Sprint(value))
		}
		return strings.NewReader(values.Encode()), "application/x-www-form-urlencoded", nil
	case configv1.Server_Mcp_HTTP_BODY_MODE_RAW:
		return strings.NewReader(mcpHTTPToolRenderTemplate(toolCfg.GetBodyTemplate(), args)), "text/plain", nil
	default:
		return nil, "", fmt.Errorf("unsupported mcp http tool body mode: %s", toolCfg.GetBodyMode().String())
	}
}

// mcpHTTPToolBodyParams 提取写入 Body 的 Tool 参数。
func mcpHTTPToolBodyParams(toolCfg *configv1.Server_Mcp_HttpTool, args map[string]any) (map[string]any, error) {
	bodyParams := make(map[string]any)
	var err error
	for _, param := range toolCfg.GetParameters() {
		if param == nil || param.GetLocation() != configv1.Server_Mcp_HTTP_PARAM_LOCATION_BODY {
			continue
		}
		var value any
		var ok bool
		value, ok, err = mcpHTTPToolParamValue(param, args)
		if err != nil {
			return nil, err
		}
		if ok {
			bodyParams[mcpHTTPToolParamTarget(param)] = value
		}
	}
	return bodyParams, nil
}

// mcpHTTPToolParamValue 读取参数值，并处理必填和默认值。
func mcpHTTPToolParamValue(param *configv1.Server_Mcp_HttpTool_Parameter, args map[string]any) (any, bool, error) {
	name := strings.TrimSpace(param.GetName())
	if name == "" {
		return nil, false, errors.New("mcp http tool parameter name is empty")
	}
	value, ok := args[name]
	if !ok || value == nil {
		if param.GetDefaultValue() != "" {
			return param.GetDefaultValue(), true, nil
		}
		if param.GetRequired() {
			return nil, false, fmt.Errorf("required mcp http tool parameter %q is missing", name)
		}
		return nil, false, nil
	}
	return value, true, nil
}

// mcpHTTPToolParamTarget 返回参数写入 HTTP 请求时使用的名称。
func mcpHTTPToolParamTarget(param *configv1.Server_Mcp_HttpTool_Parameter) string {
	target := strings.TrimSpace(param.GetTarget())
	if target != "" {
		return target
	}
	return strings.TrimSpace(param.GetName())
}

// mcpHTTPToolInputSchema 返回 HTTP Tool 的输入 JSON Schema。
func mcpHTTPToolInputSchema(toolCfg *configv1.Server_Mcp_HttpTool) any {
	if toolCfg.GetInputSchema() != nil {
		return toolCfg.GetInputSchema().AsMap()
	}
	properties := make(map[string]any, len(toolCfg.GetParameters()))
	var required []string
	for _, param := range toolCfg.GetParameters() {
		if param == nil || strings.TrimSpace(param.GetName()) == "" {
			continue
		}
		paramType := strings.TrimSpace(param.GetType())
		if paramType == "" {
			paramType = "string"
		}
		properties[param.GetName()] = map[string]any{
			"type":        paramType,
			"description": param.GetDescription(),
		}
		if param.GetRequired() {
			required = append(required, param.GetName())
		}
	}
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

// mcpHTTPToolStructuredContent 尝试将 JSON 对象响应作为结构化结果返回。
func mcpHTTPToolStructuredContent(contentType string, body []byte) any {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || !strings.Contains(mediaType, "json") || len(body) == 0 {
		return nil
	}
	var payload map[string]any
	if err = json.Unmarshal(body, &payload); err != nil {
		return nil
	}
	return payload
}

// mcpHTTPToolRenderTemplate 使用 Tool 参数渲染简单占位符模板。
func mcpHTTPToolRenderTemplate(tpl string, args map[string]any) string {
	rendered := tpl
	for key, value := range args {
		text := fmt.Sprint(value)
		rendered = strings.ReplaceAll(rendered, "{{"+key+"}}", text)
		rendered = strings.ReplaceAll(rendered, "{"+key+"}", text)
	}
	return rendered
}

// mcpHTTPToolIsAbsoluteURL 判断地址是否为绝对 HTTP URL。
func mcpHTTPToolIsAbsoluteURL(rawURL string) bool {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return parsedURL.Scheme == "http" || parsedURL.Scheme == "https"
}
