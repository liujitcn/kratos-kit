package mcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/transport/keepalive"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	// DefaultMCPServerName 是默认 MCP 服务名称。
	DefaultMCPServerName = "MCP Server"
	// DefaultMCPServerVersion 是默认 MCP 服务版本。
	DefaultMCPServerVersion = "1.0.0"
	// DefaultMCPServerAddress 是默认 MCP HTTP 监听地址。
	DefaultMCPServerAddress = ":8080"
	// DefaultMCPHandlerPath 是默认 MCP HTTP 挂载路径。
	DefaultMCPHandlerPath = "/mcp"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

// Server 封装 mcp-go 服务，并适配 Kratos transport.Server。
type Server struct {
	mu      sync.RWMutex
	started atomic.Bool

	mcpServer *server.MCPServer

	serverName    string
	serverVersion string
	serverOptions []server.ServerOption

	serverType    ServerType
	network       string
	listenAddress string
	handlerPath   string
	tlsConf       *tls.Config

	lis      net.Listener
	endpoint *url.URL

	httpServer *http.Server
	cancel     context.CancelFunc
	err        error

	shutdownTimeout time.Duration

	keepaliveServer *keepalive.Server
	enableKeepalive bool

	streamableHTTPOptions []server.StreamableHTTPOption
	jsonResponse          bool
	sseOptions            []server.SSEOption
	handlerShutdowns      []func(context.Context) error
}

// NewServer 创建 MCP 服务端。
func NewServer(opts ...ServerOption) *Server {
	srv := &Server{
		serverName:      DefaultMCPServerName,
		serverVersion:   DefaultMCPServerVersion,
		serverType:      ServerTypeStdio,
		network:         "tcp",
		listenAddress:   DefaultMCPServerAddress,
		handlerPath:     DefaultMCPHandlerPath,
		shutdownTimeout: 5 * time.Second,
		enableKeepalive: true,
	}

	srv.init(opts...)

	return srv
}

// init 应用配置并创建底层 mcp-go 服务。
func (s *Server) init(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}

	if s.enableKeepalive && s.keepaliveServer == nil {
		switch s.serverType {
		// STDIO 没有独立 HTTP 服务承载，沿用旧版逻辑创建 keepalive 端点用于服务注册。
		case ServerTypeStdio:
			s.newKeepaliveServer()
		}
	}

	options := append([]server.ServerOption{server.WithInputSchemaValidation()}, s.serverOptions...)
	s.mcpServer = server.NewMCPServer(s.serverName, s.serverVersion, options...)
}

// newKeepaliveServer 创建 MCP 服务配套的 keepalive 保活服务。
func (s *Server) newKeepaliveServer() {
	s.keepaliveServer = keepalive.NewServer(keepalive.WithServiceKind(KindMCP))
}

// Name 返回 transport 名称。
func (s *Server) Name() string {
	return KindMCP
}

// Start 启动 MCP 服务。
func (s *Server) Start(ctx context.Context) error {
	if s.serverType == ServerTypeInProcess {
		return nil
	}
	if !s.started.CompareAndSwap(false, true) {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)

	s.mu.Lock()
	s.cancel = cancel
	s.mu.Unlock()

	if s.shouldUseKeepalive() {
		s.startKeepaliveServer(runCtx)
	}

	var err error
	switch s.serverType {
	// Legacy SSE 模式同样是独立 HTTP 服务，只是 Handler 协议不同。
	case ServerTypeSSE:
		err = s.startHTTP(runCtx)
	// 独立 HTTP 模式需要先完成监听和 endpoint 计算，再阻塞 Serve。
	case ServerTypeHTTP:
		err = s.startHTTP(runCtx)
	// STDIO 模式阻塞到标准输入关闭或上下文取消。
	case ServerTypeStdio:
		err = server.NewStdioServer(s.mcpServer).Listen(runCtx, os.Stdin, os.Stdout)
	default:
		err = errors.New("unsupported mcp server type: " + string(s.serverType))
	}

	s.started.Store(false)
	cancel()
	if errors.Is(err, http.ErrServerClosed) || errors.Is(err, context.Canceled) {
		err = nil
	}
	err = errors.Join(err, s.shutdownHandlers(ctx))
	if s.shouldUseKeepalive() {
		err = errors.Join(err, s.stopKeepaliveServer(ctx))
	}

	if err != nil {
		s.setErr(err)
		return err
	}

	return nil
}

// shouldUseKeepalive 判断当前 MCP 运行形态是否需要独立 keepalive。
func (s *Server) shouldUseKeepalive() bool {
	return s.enableKeepalive && s.serverType == ServerTypeStdio && s.keepaliveServer != nil
}

// startKeepaliveServer 异步启动 keepalive 保活服务。
func (s *Server) startKeepaliveServer(ctx context.Context) {
	go func() {
		err := s.keepaliveServer.Start(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			s.setErr(fmt.Errorf("keepalive server start failed: %w", err))
			log.Error("keepalive server start failed", "error", err)
		}
	}()
}

// startHTTP 监听并运行独立 MCP HTTP 服务。
func (s *Server) startHTTP(ctx context.Context) error {
	err := s.listenAndEndpoint()
	if err != nil {
		return err
	}

	var handler http.Handler
	handler, err = s.transportHTTPHandler()
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle(s.handlerPath, handler)

	httpServer := &http.Server{
		Handler:     mux,
		TLSConfig:   s.tlsConf,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	s.mu.Lock()
	s.httpServer = httpServer
	s.mu.Unlock()

	log.Info("server listening", "kind", KindMCP, "addr", s.lis.Addr().String(), "path", s.handlerPath)

	if s.tlsConf != nil {
		return httpServer.ServeTLS(s.lis, "", "")
	}
	return httpServer.Serve(s.lis)
}

// transportHTTPHandler 根据服务类型创建对应的 MCP HTTP Handler。
func (s *Server) transportHTTPHandler() (http.Handler, error) {
	switch s.serverType {
	// SSE 模式使用 2024-11-05 Legacy SSE transport。
	case ServerTypeSSE:
		return s.SSEHandler()
	case ServerTypeHTTP:
		return s.HTTPHandler()
	default:
		return nil, errors.New("unsupported mcp http server type: " + string(s.serverType))
	}
}

// Stop 停止 MCP 服务。
func (s *Server) Stop(ctx context.Context) error {
	log.Info("server stopping", "kind", KindMCP)

	s.mu.RLock()
	cancel := s.cancel
	httpServer := s.httpServer
	s.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	shutdownCtx := ctx
	var shutdownCancel context.CancelFunc
	if _, ok := ctx.Deadline(); !ok && s.shutdownTimeout > 0 {
		shutdownCtx, shutdownCancel = context.WithTimeout(ctx, s.shutdownTimeout)
	}
	if shutdownCancel != nil {
		defer shutdownCancel()
	}
	// 先关闭 SSE 会话和会话清理协程，再等待 HTTP 请求退出。
	err := s.shutdownHandlers(shutdownCtx)
	if httpServer != nil {
		err = errors.Join(err, httpServer.Shutdown(shutdownCtx))
	}

	if s.shouldUseKeepalive() {
		keepaliveErr := s.stopKeepaliveServer(ctx)
		if keepaliveErr != nil {
			err = errors.Join(err, keepaliveErr)
		}
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.setErr(err)
		return err
	}

	s.started.Store(false)
	log.Info("server stopped", "kind", KindMCP)

	return nil
}

// shutdownHandlers 释放已创建 Handler 持有的会话和后台任务。
func (s *Server) shutdownHandlers(ctx context.Context) error {
	s.mu.Lock()
	shutdowns := s.handlerShutdowns
	s.handlerShutdowns = nil
	s.mu.Unlock()
	var err error
	for _, shutdown := range shutdowns {
		err = errors.Join(err, shutdown(ctx))
	}
	return err
}

// stopKeepaliveServer 停止 keepalive 保活服务。
func (s *Server) stopKeepaliveServer(ctx context.Context) error {
	if s.keepaliveServer == nil {
		return nil
	}
	err := s.keepaliveServer.Stop(ctx)
	if err != nil {
		log.Error("keepalive server stop failed", "error", err)
	}
	return err
}

// Endpoint 返回独立 HTTP MCP 服务的注册端点。
func (s *Server) Endpoint() (*url.URL, error) {
	s.mu.RLock()
	endpoint := s.endpoint
	s.mu.RUnlock()
	if endpoint != nil {
		return endpoint, nil
	}

	if s.shouldUseKeepalive() {
		return s.keepaliveServer.Endpoint()
	}

	if s.serverType != ServerTypeHTTP && s.serverType != ServerTypeSSE {
		return &url.URL{Scheme: KindMCP}, nil
	}

	if err := s.listenAndEndpoint(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.endpoint, nil
}

// listenAndEndpoint 创建监听器并计算服务注册端点。
func (s *Server) listenAndEndpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var err error
	if s.lis == nil {
		var lis net.Listener
		lis, err = net.Listen(s.network, s.listenAddress)
		if err != nil {
			return err
		}
		s.lis = lis
	}

	if s.endpoint == nil {
		var endpoint *url.URL
		endpoint, err = newHTTPEndpoint(s.listenAddress, s.handlerPath, s.lis, s.tlsConf != nil)
		if err != nil {
			return err
		}
		s.endpoint = endpoint
	}

	return nil
}

// MCPServer 返回底层 mcp-go 服务实例。
func (s *Server) MCPServer() *server.MCPServer {
	if s == nil {
		return nil
	}
	return s.mcpServer
}

// RegisterHandler 注册原始 MCP Tool 处理器。
func (s *Server) RegisterHandler(tool mcp.Tool, handler server.ToolHandlerFunc) error {
	return s.AddTool(tool, handler)
}

// AddTool 注册原始 MCP Tool 处理器。
func (s *Server) AddTool(tool mcp.Tool, handler server.ToolHandlerFunc) error {
	if s == nil || s.mcpServer == nil {
		return errors.New("mcp server is nil")
	}
	s.mcpServer.AddTool(tool, handler)
	return nil
}

// HTTPHandler 创建可挂载到已有 HTTP 服务的 Streamable HTTP MCP Handler。
func (s *Server) HTTPHandler(opts ...server.StreamableHTTPOption) (http.Handler, error) {
	if s == nil || s.mcpServer == nil {
		return nil, errors.New("mcp server is nil")
	}

	options := append([]server.StreamableHTTPOption{server.WithStateful(true), server.WithEndpointPath(s.handlerPath)}, s.streamableHTTPOptions...)
	options = append(options, opts...)
	handler := server.NewStreamableHTTPServer(s.mcpServer, options...)
	s.mu.Lock()
	s.handlerShutdowns = append(s.handlerShutdowns, handler.Shutdown)
	s.mu.Unlock()
	if s.jsonResponse {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				// 隐藏 Flusher，使带通知的 POST 也保持 JSON 响应，GET 仍可使用事件流。
				w = struct{ http.ResponseWriter }{w}
			}
			handler.ServeHTTP(w, r)
		}), nil
	}
	return handler, nil
}

// SSEHandler 创建可挂载到已有 HTTP 服务的 Legacy SSE MCP Handler。
func (s *Server) SSEHandler(opts ...server.SSEOption) (http.Handler, error) {
	if s == nil || s.mcpServer == nil {
		return nil, errors.New("mcp server is nil")
	}

	options := append([]server.SSEOption{
		server.WithSSEEndpoint(s.handlerPath), server.WithMessageEndpoint(s.handlerPath), server.WithUseFullURLForMessageEndpoint(false),
	}, s.sseOptions...)
	options = append(options, opts...)
	handler := server.NewSSEServer(s.mcpServer, options...)
	s.mu.Lock()
	s.handlerShutdowns = append(s.handlerShutdowns, func(ctx context.Context) error {
		handler.CloseSessions()
		return handler.Shutdown(ctx)
	})
	s.mu.Unlock()
	// 沿用同一路径承载 SSE 连接与消息 POST，避免 SDK 的路径匹配优先命中 SSE。
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodOptions:
			handler.SSEHandler().ServeHTTP(w, r)
		case http.MethodPost:
			handler.MessageHandler().ServeHTTP(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}), nil
}

// setErr 累积服务运行或停止期间的错误。
func (s *Server) setErr(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.err = errors.Join(s.err, err)
	s.mu.Unlock()
}
