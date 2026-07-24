package mcp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	kratosTransport "github.com/go-kratos/kratos/v3/transport"
	"github.com/liujitcn/kratos-kit/transport/keepalive"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
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
	_ kratosTransport.Server     = (*Server)(nil)
	_ kratosTransport.Endpointer = (*Server)(nil)
)

// Server 封装官方 MCP SDK 服务，并适配 Kratos transport.Server。
type Server struct {
	mu      sync.RWMutex
	started atomic.Bool

	mcpServer *mcpsdk.Server

	serverName    string
	serverVersion string
	serverOptions *mcpsdk.ServerOptions

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

	streamableHTTPOptions *mcpsdk.StreamableHTTPOptions
	sseOptions            *mcpsdk.SSEOptions
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

// Name 返回 transport 名称。
func (s *Server) Name() string {
	return KindMCP
}

// Start 启动 MCP 服务。
func (s *Server) Start(ctx context.Context) error {
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
	// STDIO 模式用于命令行 MCP Server，Run 会阻塞到连接关闭或上下文取消。
	case ServerTypeStdio:
		err = s.mcpServer.Run(runCtx, &mcpsdk.StdioTransport{})
	case ServerTypeInProcess:
		err = nil
	default:
		err = errors.New("unsupported mcp server type: " + string(s.serverType))
	}

	s.started.Store(false)
	cancel()
	if s.shouldUseKeepalive() {
		_ = s.stopKeepaliveServer(ctx)
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) {
		s.setErr(err)
		return err
	}

	return nil
}

// Stop 停止 MCP 服务。
func (s *Server) Stop(ctx context.Context) error {
	if !s.started.Load() {
		return nil
	}

	log.Info("server stopping", "kind", KindMCP)

	s.mu.RLock()
	cancel := s.cancel
	httpServer := s.httpServer
	s.mu.RUnlock()

	if cancel != nil {
		cancel()
	}

	var err error
	if httpServer != nil {
		shutdownCtx := ctx
		var shutdownCancel context.CancelFunc
		if _, ok := ctx.Deadline(); !ok && s.shutdownTimeout > 0 {
			shutdownCtx, shutdownCancel = context.WithTimeout(ctx, s.shutdownTimeout)
		}
		if shutdownCancel != nil {
			defer shutdownCancel()
		}
		err = httpServer.Shutdown(shutdownCtx)
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

// MCPServer 返回底层官方 MCP SDK 服务实例。
func (s *Server) MCPServer() *mcpsdk.Server {
	if s == nil {
		return nil
	}
	return s.mcpServer
}

// AddTool 注册原始 MCP Tool 处理器。
func (s *Server) AddTool(tool *mcpsdk.Tool, handler mcpsdk.ToolHandler) error {
	if s == nil || s.mcpServer == nil {
		return errors.New("mcp server is nil")
	}
	s.mcpServer.AddTool(tool, handler)
	return nil
}

// RegisterHandler 注册原始 MCP Tool 处理器。
func (s *Server) RegisterHandler(tool *mcpsdk.Tool, handler mcpsdk.ToolHandler) error {
	return s.AddTool(tool, handler)
}

// HTTPHandler 创建可挂载到已有 HTTP 服务的 Streamable HTTP MCP Handler。
func (s *Server) HTTPHandler(opts ...*mcpsdk.StreamableHTTPOptions) (http.Handler, error) {
	if s == nil || s.mcpServer == nil {
		return nil, errors.New("mcp server is nil")
	}

	options := cloneStreamableHTTPOptions(s.streamableHTTPOptions)
	for _, opt := range opts {
		options = mergeStreamableHTTPOptions(options, opt)
	}

	return mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return s.mcpServer
	}, options), nil
}

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

	impl := &mcpsdk.Implementation{
		Name:    s.serverName,
		Version: s.serverVersion,
	}
	s.mcpServer = mcpsdk.NewServer(impl, s.serverOptions)
}

func (s *Server) startHTTP(ctx context.Context) error {
	if err := s.listenAndEndpoint(); err != nil {
		return err
	}

	handler, err := s.transportHTTPHandler()
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

// SSEHandler 创建可挂载到已有 HTTP 服务的 Legacy SSE MCP Handler。
func (s *Server) SSEHandler(opts ...*mcpsdk.SSEOptions) (http.Handler, error) {
	if s == nil || s.mcpServer == nil {
		return nil, errors.New("mcp server is nil")
	}

	options := cloneSSEOptions(s.sseOptions)
	for _, opt := range opts {
		options = mergeSSEOptions(options, opt)
	}

	return mcpsdk.NewSSEHandler(func(*http.Request) *mcpsdk.Server {
		return s.mcpServer
	}, options), nil
}

// newKeepaliveServer 创建 MCP 服务配套的 keepalive 保活服务。
func (s *Server) newKeepaliveServer() {
	s.keepaliveServer = keepalive.NewServer(keepalive.WithServiceKind(KindMCP))
}

// shouldUseKeepalive 判断当前 MCP 运行形态是否需要独立 keepalive。
func (s *Server) shouldUseKeepalive() bool {
	return s.enableKeepalive && s.serverType == ServerTypeStdio && s.keepaliveServer != nil
}

// startKeepaliveServer 异步启动 keepalive 保活服务。
func (s *Server) startKeepaliveServer(ctx context.Context) {
	go func() {
		if keepaliveErr := s.keepaliveServer.Start(ctx); keepaliveErr != nil && !errors.Is(keepaliveErr, context.Canceled) {
			s.setErr(errors.New("keepalive server start failed: " + keepaliveErr.Error()))
			log.Error("keepalive server start failed", "error", keepaliveErr)
		}
	}()
}

// stopKeepaliveServer 停止 keepalive 保活服务。
func (s *Server) stopKeepaliveServer(ctx context.Context) error {
	if s.keepaliveServer == nil {
		return nil
	}
	if keepaliveErr := s.keepaliveServer.Stop(ctx); keepaliveErr != nil {
		log.Error("keepalive server stop failed", "error", keepaliveErr)
		return keepaliveErr
	}
	return nil
}

func (s *Server) listenAndEndpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lis == nil {
		lis, err := net.Listen(s.network, s.listenAddress)
		if err != nil {
			return err
		}
		s.lis = lis
	}

	if s.endpoint == nil {
		endpoint, err := newHTTPEndpoint(s.listenAddress, s.handlerPath, s.lis, s.tlsConf != nil)
		if err != nil {
			return err
		}
		s.endpoint = endpoint
	}

	return nil
}

func (s *Server) setErr(err error) {
	if err == nil {
		return
	}
	s.mu.Lock()
	s.err = errors.Join(s.err, err)
	s.mu.Unlock()
}
