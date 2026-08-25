package mcp

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// LoadToolFromJsonString 从 JSON 字符串加载 MCP Tool。
func LoadToolFromJsonString(jsonStr string) (*mcp.Tool, error) {
	var tool mcp.Tool
	if err := json.Unmarshal([]byte(jsonStr), &tool); err != nil {
		return nil, fmt.Errorf("JSON 反序列化失败：%w", err)
	}
	if err := ValidateToolInputSchema(tool.InputSchema); err != nil {
		return nil, err
	}
	return &tool, nil
}

// ValidateToolInputSchema 校验 Tool InputSchema 的基础结构。
func ValidateToolInputSchema(schema any) error {
	if schema == nil {
		return fmt.Errorf("InputSchema 不能为空")
	}

	var raw map[string]any
	data, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("InputSchema 序列化失败：%w", err)
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("InputSchema 反序列化失败：%w", err)
	}

	schemaType, ok := raw["type"].(string)
	if !ok || schemaType == "" {
		return fmt.Errorf("InputSchema.type 不能为空（必须是 object）")
	}
	if schemaType != "object" {
		return fmt.Errorf("InputSchema.type 必须为 object（当前：%s）", schemaType)
	}

	return nil
}

func cloneServerOptions(opts *mcp.ServerOptions) *mcp.ServerOptions {
	if opts == nil {
		return nil
	}
	cloned := *opts
	return &cloned
}

func cloneStreamableHTTPOptions(opts *mcp.StreamableHTTPOptions) *mcp.StreamableHTTPOptions {
	if opts == nil {
		return nil
	}
	cloned := *opts
	return &cloned
}

func mergeStreamableHTTPOptions(base, override *mcp.StreamableHTTPOptions) *mcp.StreamableHTTPOptions {
	if override == nil {
		return cloneStreamableHTTPOptions(base)
	}
	if base == nil {
		merged := *override
		return &merged
	}

	merged := *base
	if override.Stateless {
		merged.Stateless = true
	}
	if override.JSONResponse {
		merged.JSONResponse = true
	}
	if override.Logger != nil {
		merged.Logger = override.Logger
	}
	if override.EventStore != nil {
		merged.EventStore = override.EventStore
	}
	if override.SessionTimeout != 0 {
		merged.SessionTimeout = override.SessionTimeout
	}
	if override.DisableLocalhostProtection {
		merged.DisableLocalhostProtection = true
	}
	if override.CrossOriginProtection != nil {
		merged.CrossOriginProtection = override.CrossOriginProtection
	}
	return &merged
}

func cloneSSEOptions(opts *mcp.SSEOptions) *mcp.SSEOptions {
	if opts == nil {
		return nil
	}
	cloned := *opts
	return &cloned
}

func mergeSSEOptions(base, override *mcp.SSEOptions) *mcp.SSEOptions {
	if override == nil {
		return cloneSSEOptions(base)
	}
	if base == nil {
		merged := *override
		return &merged
	}
	merged := *base
	if override.DisableLocalhostProtection {
		merged.DisableLocalhostProtection = true
	}
	return &merged
}

func normalizeHandlerPath(path string) string {
	if path == "" {
		return DefaultMCPHandlerPath
	}
	if !strings.HasPrefix(path, "/") {
		return "/" + path
	}
	return path
}

func newHTTPEndpoint(address, handlerPath string, lis net.Listener, secure bool) (*url.URL, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if lis == nil {
			return nil, err
		}
		host = ""
	}

	if lis != nil {
		if tcpAddr, ok := lis.Addr().(*net.TCPAddr); ok {
			port = fmt.Sprintf("%d", tcpAddr.Port)
			if host == "" && tcpAddr.IP != nil && !tcpAddr.IP.IsUnspecified() {
				host = tcpAddr.IP.String()
			}
		}
	}

	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "localhost"
	}

	scheme := "http"
	if secure {
		scheme = "https"
	}

	return &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(strings.Trim(host, "[]"), port),
		Path:   normalizeHandlerPath(handlerPath),
	}, nil
}
