package keepalive

import (
	"crypto/tls"
	"net/url"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type ServerOption func(o *Server)

// WithTLSConfig TLS配置
func WithTLSConfig(tlsConf *tls.Config) ServerOption {
	return func(s *Server) {
		if tlsConf != nil {
			s.grpcOpts = append(s.grpcOpts, grpc.Creds(credentials.NewTLS(tlsConf)))
		}
	}
}

func WithNetwork(network string) ServerOption {
	return func(s *Server) {
		s.network = network
	}
}

func WithAddress(addr string) ServerOption {
	return func(s *Server) {
		s.address = addr
	}
}

// WithHost 设置保活 endpoint 使用的主机地址。
func WithHost(host string) ServerOption {
	return func(s *Server) {
		s.host = host
	}
}

// WithInterface 设置保活 endpoint 自动探测时使用的网卡名称。
func WithInterface(name string) ServerOption {
	return func(s *Server) {
		s.interfaceName = name
	}
}

func WithEndpoint(endpoint *url.URL) ServerOption {
	return func(s *Server) {
		s.endpoint = endpoint
	}
}

func WithServiceKind(name string) ServerOption {
	return func(s *Server) {
		s.serviceKind = name
	}
}
