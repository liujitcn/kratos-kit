package mcp

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/liujitcn/kratos-kit/transport/keepalive"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ServerType 表示 MCP 服务的运行形态。
type ServerType string

const (
	// ServerTypeSSE 表示独立监听端口的 Legacy SSE MCP 服务。
	ServerTypeSSE ServerType = "SSE"
	// ServerTypeHTTP 表示独立监听端口的 Streamable HTTP MCP 服务。
	ServerTypeHTTP ServerType = "HTTP"
	// ServerTypeStdio 表示使用标准输入输出运行的 MCP 服务。
	ServerTypeStdio ServerType = "STDIO"
	// ServerTypeInProcess 表示进程内 MCP 服务，可挂载到外部 HTTP 服务，不单独监听端口。
	ServerTypeInProcess ServerType = "IN_PROCESS"
)

// ServerOption 定义 MCP 服务配置选项。
type ServerOption func(o *Server)

// WithServerName 设置 MCP 服务名称。
func WithServerName(name string) ServerOption {
	return func(s *Server) {
		s.serverName = name
	}
}

// WithServerVersion 设置 MCP 服务版本。
func WithServerVersion(version string) ServerOption {
	return func(s *Server) {
		s.serverVersion = version
	}
}

// WithServerOptions 设置官方 MCP SDK 服务选项。
func WithServerOptions(opts *mcp.ServerOptions) ServerOption {
	return func(s *Server) {
		s.serverOptions = cloneServerOptions(opts)
	}
}

// WithMCPServerOptions 设置官方 MCP SDK 服务选项。
func WithMCPServerOptions(opts *mcp.ServerOptions) ServerOption {
	return WithServerOptions(opts)
}

// WithServerType 设置 MCP 服务运行形态。
func WithServerType(serverType ServerType) ServerOption {
	return func(s *Server) {
		s.serverType = serverType
	}
}

// WithMCPServeType 设置 MCP 服务运行形态。
func WithMCPServeType(serverType ServerType) ServerOption {
	return WithServerType(serverType)
}

// WithListenAddress 设置独立 HTTP 服务监听地址。
func WithListenAddress(addr string) ServerOption {
	return func(s *Server) {
		s.listenAddress = addr
	}
}

// WithMCPServeAddress 设置独立 HTTP 服务监听地址。
func WithMCPServeAddress(addr string) ServerOption {
	return WithListenAddress(addr)
}

// WithNetwork 设置独立 HTTP 服务监听网络。
func WithNetwork(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

// WithListener 设置独立 HTTP 服务使用的监听器。
func WithListener(lis net.Listener) ServerOption {
	return func(s *Server) {
		s.lis = lis
	}
}

// WithHandlerPath 设置独立 HTTP 服务挂载 MCP Handler 的路径。
func WithHandlerPath(path string) ServerOption {
	return func(s *Server) {
		s.handlerPath = normalizeHandlerPath(path)
	}
}

// WithTLSConfig 设置独立 HTTP 服务 TLS 配置。
func WithTLSConfig(tlsConf *tls.Config) ServerOption {
	return func(s *Server) {
		s.tlsConf = tlsConf
	}
}

// WithEnableKeepAlive 设置是否启用 keepalive 保活服务。
func WithEnableKeepAlive(enable bool) ServerOption {
	return func(s *Server) {
		s.enableKeepalive = enable
	}
}

// WithKeepAliveServer 设置 MCP 服务配套使用的 keepalive 保活服务实例。
func WithKeepAliveServer(server *keepalive.Server) ServerOption {
	return func(s *Server) {
		s.keepaliveServer = server
	}
}

// WithShutdownTimeout 设置 Stop 在调用方未给 deadline 时使用的默认超时时间。
func WithShutdownTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.shutdownTimeout = timeout
	}
}

// WithStreamableHTTPOptions 设置 Streamable HTTP Handler 选项。
func WithStreamableHTTPOptions(opts *mcp.StreamableHTTPOptions) ServerOption {
	return func(s *Server) {
		s.streamableHTTPOptions = mergeStreamableHTTPOptions(s.streamableHTTPOptions, opts)
	}
}

// WithSSEOptions 设置 Legacy SSE Handler 选项。
func WithSSEOptions(opts *mcp.SSEOptions) ServerOption {
	return func(s *Server) {
		s.sseOptions = cloneSSEOptions(opts)
	}
}
