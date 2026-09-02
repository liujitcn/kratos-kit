package redact

import "google.golang.org/grpc"

// ServerStreamRedactor 包装 gRPC 服务端流并在发送前脱敏响应。
type ServerStreamRedactor[Res any] struct {
	grpc.ServerStreamingServer[Res]
	Resolver PolicyResolver
}

// Send 脱敏响应后发送给客户端。
func (s *ServerStreamRedactor[Res]) Send(message *Res) error {
	ApplyWith(s.Context(), s.Resolver, message)
	return s.ServerStreamingServer.Send(message)
}

// BidiStreamRedactor 包装 gRPC 双向流并在发送前脱敏响应。
type BidiStreamRedactor[Req any, Res any] struct {
	grpc.BidiStreamingServer[Req, Res]
	Resolver PolicyResolver
}

// Send 脱敏响应后发送给客户端。
func (s *BidiStreamRedactor[Req, Res]) Send(message *Res) error {
	ApplyWith(s.Context(), s.Resolver, message)
	return s.BidiStreamingServer.Send(message)
}

// ClientStreamRedactor 包装 gRPC 客户端流并在发送前脱敏响应。
type ClientStreamRedactor[Req any, Res any] struct {
	grpc.ClientStreamingServer[Req, Res]
	Resolver PolicyResolver
}

// SendAndClose 脱敏响应后发送给客户端并关闭流。
func (s *ClientStreamRedactor[Req, Res]) SendAndClose(message *Res) error {
	ApplyWith(s.Context(), s.Resolver, message)
	return s.ClientStreamingServer.SendAndClose(message)
}
