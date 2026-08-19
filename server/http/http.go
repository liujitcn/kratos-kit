package http

import (
	"crypto/tls"
	"net/http/pprof"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	midRateLimit "github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	kratosHttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/gorilla/handlers"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	httpResponse "github.com/liujitcn/kratos-kit/server/http/internal/response"
	httpRequestID "github.com/liujitcn/kratos-kit/server/http/middleware/requestid"
	kitTracing "github.com/liujitcn/kratos-kit/tracing"
	"github.com/liujitcn/kratos-kit/utils"
)

// CreateHttpServer 创建 HTTP 服务端。
func CreateHttpServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Server, error) {
	options, err := initHttpServerConfig(cfg, mds...)
	if err != nil {
		return nil, err
	}
	options = append(options, kratosHttp.ResponseEncoder(httpResponse.ProtoJSONEncoder))

	srv := kratosHttp.NewServer(options...)

	if cfg != nil && cfg.Server != nil && cfg.Server.Http != nil && cfg.Server.Http.GetEnablePprof() {
		registerHttpPprof(srv)
	}

	return srv, nil
}

// initHttpServerConfig 初始化 HTTP 服务配置。
func initHttpServerConfig(cfg *configv1.Bootstrap, mds ...middleware.Middleware) ([]kratosHttp.ServerOption, error) {
	if cfg == nil || cfg.Server == nil || cfg.Server.Http == nil {
		return nil, nil
	}

	httpCfg := cfg.Server.Http

	options := make([]kratosHttp.ServerOption, 0)

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
		options = append(options, kratosHttp.Filter(handlers.CORS(corsOptions...)))
	}

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := httpCfg.Middleware
	transportMiddlewares := []middleware.Middleware{httpRequestID.Server()}
	if middlewareCfg != nil {
		if middlewareCfg.GetEnableRecovery() {
			transportMiddlewares = append(transportMiddlewares, recovery.Recovery())
		}
		if middlewareCfg.GetEnableTracing() {
			transportMiddlewares = append(transportMiddlewares, kitTracing.Server())
		}
		if middlewareCfg.GetEnableMetadata() {
			transportMiddlewares = append(transportMiddlewares, metadata.Server())
		}
		if middlewareCfg.Limiter != nil {
			transportMiddlewares = append(transportMiddlewares, midRateLimit.Server())
		}
	}
	// 传输层中间件固定在业务中间件之前，保证请求上下文和基础防护先完成。
	mds = append(transportMiddlewares, mds...)

	options = append(options, kratosHttp.Middleware(mds...))

	if httpCfg.Network != "" {
		options = append(options, kratosHttp.Network(httpCfg.Network))
	}
	if httpCfg.Addr != "" {
		options = append(options, kratosHttp.Address(httpCfg.Addr))
	}
	if httpCfg.Timeout != nil {
		options = append(options, kratosHttp.Timeout(httpCfg.Timeout.AsDuration()))
	}

	if httpCfg.Tls != nil {
		var tlsCfg *tls.Config
		var err error

		if tlsCfg, err = utils.LoadServerTlsConfig(httpCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, kratosHttp.TLSConfig(tlsCfg))
		}
	}

	return options, nil
}

// registerHttpPprof 注册 pprof 路由。
func registerHttpPprof(s *kratosHttp.Server) {
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
