package mcp

// NewHandler 创建可挂载到现有 HTTP 服务的 MCP 服务端。
func NewHandler(opts ...ServerOption) *Server {
	options := make([]ServerOption, 0, len(opts)+1)
	options = append(options, opts...)
	options = append(options, WithServerType(ServerTypeInProcess))
	return NewServer(options...)
}
