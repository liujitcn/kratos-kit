package sse

// NewHandler 创建可挂载到现有 HTTP 服务的 SSE 处理器。
func NewHandler(opts ...ServerOption) *Server {
	return newServer(opts...)
}
