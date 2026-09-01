package http

import (
	"crypto/tls"
	stdhttp "net/http"
	"net/http/pprof"
	"strings"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	"github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/gorilla/handlers"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/server/http/internal/response"
	"github.com/liujitcn/kratos-kit/server/http/middleware/requestid"
	"github.com/liujitcn/kratos-kit/server/http/middleware/timeout"
	"github.com/liujitcn/kratos-kit/tracing"
	"github.com/liujitcn/kratos-kit/utils"
)

// CreateHttpServer 创建 HTTP 服务端。
func CreateHttpServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*http.Server, error) {
	options, err := initHttpServerConfig(cfg, mds...)
	if err != nil {
		return nil, err
	}
	options = append(options, http.ResponseEncoder(response.ProtoJSONEncoder))

	srv := http.NewServer(options...)

	if cfg != nil && cfg.Server != nil && cfg.Server.Http != nil && cfg.Server.Http.GetEnablePprof() {
		registerHttpPprof(srv)
	}

	return srv, nil
}

// initHttpServerConfig 初始化 HTTP 服务配置。
func initHttpServerConfig(cfg *configv1.Bootstrap, mds ...middleware.Middleware) ([]http.ServerOption, error) {
	if cfg == nil || cfg.Server == nil || cfg.Server.Http == nil {
		return nil, nil
	}

	httpCfg := cfg.Server.Http

	options := make([]http.ServerOption, 0)
	filters := make([]http.FilterFunc, 0, 3)

	if httpCfg.Cors != nil {
		corsOptions := make([]handlers.CORSOption, 0, 6)
		if len(httpCfg.Cors.Headers) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedHeaders(httpCfg.Cors.Headers))
		}
		if len(httpCfg.Cors.Methods) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedMethods(httpCfg.Cors.Methods))
		}
		if len(httpCfg.Cors.Origins) > 0 {
			corsOptions = append(corsOptions, handlers.AllowedOrigins(httpCfg.Cors.Origins))
		}
		if len(httpCfg.Cors.ExposedHeaders) > 0 {
			corsOptions = append(corsOptions, handlers.ExposedHeaders(httpCfg.Cors.ExposedHeaders))
		}
		if httpCfg.Cors.AllowCredentials {
			corsOptions = append(corsOptions, handlers.AllowCredentials())
		}
		if httpCfg.Cors.MaxAgeSeconds > 0 {
			corsOptions = append(corsOptions, handlers.MaxAge(int(httpCfg.Cors.MaxAgeSeconds)))
		}
		filters = append(filters, handlers.CORS(corsOptions...))
	}
	if httpCfg.MaxBodyBytes > 0 {
		maxBodyBytes := httpCfg.MaxBodyBytes
		filters = append(filters, func(next stdhttp.Handler) stdhttp.Handler {
			return stdhttp.HandlerFunc(func(writer stdhttp.ResponseWriter, request *stdhttp.Request) {
				if request.Body != nil {
					request.Body = stdhttp.MaxBytesReader(writer, request.Body, maxBodyBytes)
				}
				next.ServeHTTP(writer, request)
			})
		})
	}
	if httpCfg.Timeout != nil && httpCfg.Timeout.AsDuration() > 0 {
		filters = append(filters, timeout.Middleware(httpCfg.Timeout.AsDuration(), timeout.WithSkipFunc(streamingRequest)))
	}
	if len(filters) > 0 {
		options = append(options, http.Filter(filters...))
	}

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := httpCfg.Middleware
	transportMiddlewares := []middleware.Middleware{requestid.Server()}
	if middlewareCfg != nil {
		if middlewareCfg.GetEnableRecovery() {
			transportMiddlewares = append(transportMiddlewares, recovery.Recovery())
		}
		if middlewareCfg.GetEnableTracing() {
			transportMiddlewares = append(transportMiddlewares, tracing.Server())
		}
		if middlewareCfg.GetEnableMetadata() {
			transportMiddlewares = append(transportMiddlewares, metadata.Server())
		}
		if middlewareCfg.Limiter != nil {
			transportMiddlewares = append(transportMiddlewares, ratelimit.Server())
		}
	}
	// 传输层中间件固定在业务中间件之前，保证请求上下文和基础防护先完成。
	mds = append(transportMiddlewares, mds...)

	options = append(options, http.Middleware(mds...))

	if httpCfg.Network != "" {
		options = append(options, http.Network(httpCfg.Network))
	}
	if httpCfg.Addr != "" {
		options = append(options, http.Address(httpCfg.Addr))
	}
	// timeout 已由可跳过流式请求的 HTTP 过滤器处理，传输层不设置全局 deadline。
	options = append(options, http.Timeout(0))

	if httpCfg.Tls != nil {
		var tlsCfg *tls.Config
		var err error

		if tlsCfg, err = utils.LoadServerTlsConfig(httpCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, http.TLSConfig(tlsCfg))
		}
	}

	return options, nil
}

// streamingRequest 判断当前路径是否需要保留长连接语义。
func streamingRequest(request *stdhttp.Request) bool {
	path := request.URL.Path
	if path == "/events" || path == "/mcp" || strings.HasPrefix(path, "/events/") || strings.HasPrefix(path, "/mcp/") {
		return true
	}
	return strings.HasPrefix(path, "/api/v1/base/ai/session/") && strings.Contains(path, "/message")
}

// registerHttpPprof 注册 pprof 路由。
func registerHttpPprof(s *http.Server) {
	s.HandleFunc("/debug/pprof", pprof.Index)

	s.HandleFunc("/debug/cmdline", pprof.Cmdline)
	s.HandleFunc("/debug/profile", pprof.Profile)
	s.HandleFunc("/debug/symbol", pprof.Symbol)
	s.HandleFunc("/debug/trace", pprof.Trace)

	s.HandleFunc("/debug/allocs", pprof.Handler("allocs").ServeHTTP)
	s.HandleFunc("/debug/block", pprof.Handler("block").ServeHTTP)
	s.HandleFunc("/debug/goroutine", pprof.Handler("goroutine").ServeHTTP)
	s.HandleFunc("/debug/heap", pprof.Handler("heap").ServeHTTP)
	s.HandleFunc("/debug/mutex", pprof.Handler("mutex").ServeHTTP)
	s.HandleFunc("/debug/threadcreate", pprof.Handler("threadcreate").ServeHTTP)
}
