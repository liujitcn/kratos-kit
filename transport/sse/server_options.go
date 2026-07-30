package sse

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/go-kratos/kratos/v3/encoding"
)

const DefaultBufferSize = 1024

type ServerOption func(o *Server)

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

func WithPath(path string) ServerOption {
	return func(s *Server) {
		s.path = path
	}
}

func WithTimeout(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.timeout = timeout
	}
}

func WithTLSConfig(c *tls.Config) ServerOption {
	return func(o *Server) {
		o.tlsConf = c
	}
}

func WithListener(lis net.Listener) ServerOption {
	return func(s *Server) {
		s.lis = lis
	}
}

func WithBufferSize(size int) ServerOption {
	return func(s *Server) {
		s.bufferSize = size
	}
}

func WithCodec(c string) ServerOption {
	return func(s *Server) {
		s.codec = encoding.GetCodec(c)
	}
}

func WithEncodeBase64(enable bool) ServerOption {
	return func(s *Server) {
		s.encodeBase64 = enable
	}
}

func WithAutoStream(enable bool) ServerOption {
	return func(s *Server) {
		s.autoStream = enable
	}
}

func WithAutoReply(enable bool) ServerOption {
	return func(s *Server) {
		s.autoReplay = enable
	}
}

// WithAutoReplay 设置新订阅者是否按 Last-Event-ID 重放历史事件。
func WithAutoReplay(enable bool) ServerOption {
	return func(s *Server) {
		s.autoReplay = enable
	}
}

func WithSplitData(enable bool) ServerOption {
	return func(s *Server) {
		s.splitData = enable
	}
}

func WithHeaders(headers map[string]string) ServerOption {
	return func(s *Server) {
		s.headers = headers
	}
}

// WithCORSAllowOrigin 设置 SSE 响应允许的跨域来源。
func WithCORSAllowOrigin(origin string) ServerOption {
	return func(s *Server) {
		s.corsAllowOrigin = origin
	}
}

func WithSubscriberFunction(sub SubscriberFunction) ServerOption {
	return func(s *Server) {
		s.subscribeFunc = sub
	}
}

func WithUnSubscriberFunction(unsub SubscriberFunction) ServerOption {
	return func(s *Server) {
		s.unsubscribeFunc = unsub
	}
}

func WithEventTTL(timeout time.Duration) ServerOption {
	return func(s *Server) {
		s.eventTTL = timeout
	}
}

func WithStreamIdKey(key string) ServerOption {
	return func(s *Server) {
		s.streamIdKey = key
		s.streamIDResolver = ResolveStreamIDFromQuery(key)
	}
}

func WithStreamIDResolver(resolver StreamIDResolver) ServerOption {
	return func(s *Server) {
		s.streamIDResolver = resolver
	}
}

// WithTokenExtractor 设置 SSE 请求的认证令牌提取函数。
func WithTokenExtractor(extractor TokenExtractor) ServerOption {
	return func(s *Server) {
		s.tokenExtractor = extractor
	}
}

// WithAuthorizeFunc 设置 SSE 订阅请求的授权函数。
func WithAuthorizeFunc(authorize AuthorizeFunc) ServerOption {
	return func(s *Server) {
		s.authorizeFunc = authorize
	}
}
