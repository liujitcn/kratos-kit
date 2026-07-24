package rpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http/pprof"
	"strings"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/registry"
	"github.com/go-kratos/kratos/v3/selector"
	selectorFilter "github.com/go-kratos/kratos/v3/selector/filter"
	selectorP2c "github.com/go-kratos/kratos/v3/selector/p2c"
	selectorRandom "github.com/go-kratos/kratos/v3/selector/random"
	selectorWrr "github.com/go-kratos/kratos/v3/selector/wrr"
	kratosHttp "github.com/go-kratos/kratos/v3/transport/http"
	"github.com/gorilla/handlers"
	"github.com/liujitcn/kratos-kit/auth/authn/engine/jwt"
	authnMiddleware "github.com/liujitcn/kratos-kit/auth/authn/middleware"
	"github.com/liujitcn/kratos-kit/rpc/middleware/requestid"
	kitTracing "github.com/liujitcn/kratos-kit/tracing"
	"github.com/liujitcn/kratos-kit/utils"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	midRateLimit "github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/rpc/middleware/validate"
)

// CreateHttpClient 创建HTTP客户端
func CreateHttpClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Client, error) {
	var err error
	var options []kratosHttp.ClientOption

	options = append(options, kratosHttp.WithDiscovery(r))

	var endpoint string
	if strings.HasPrefix(serviceName, "discovery:///") {
		endpoint = serviceName
	} else {
		endpoint = "discovery:///" + serviceName
	}
	options = append(options, kratosHttp.WithEndpoint(endpoint))

	options, err = initHttpClientConfig(cfg, options, mds...)
	if err != nil {
		return nil, fmt.Errorf("init http client config failed: %w", err)
	}

	var conn *kratosHttp.Client
	conn, err = kratosHttp.NewClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("dial http client [%s] failed: %w", serviceName, err)
	}

	return conn, nil
}

func initHttpClientConfig(cfg *configv1.Bootstrap, options []kratosHttp.ClientOption, mds ...middleware.Middleware) ([]kratosHttp.ClientOption, error) {
	if cfg == nil || cfg.Client == nil || cfg.Client.Http == nil {
		return options, nil
	}

	httpCfg := cfg.Client.Http

	timeout := defaultTimeout
	if httpCfg.Timeout != nil {
		timeout = httpCfg.Timeout.AsDuration()
	}
	options = append(options, kratosHttp.WithTimeout(timeout))

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := httpCfg.Middleware

	if middlewareCfg != nil {
		if middlewareCfg.GetEnableRecovery() {
			mds = append(mds, recovery.Recovery())
		}
		if middlewareCfg.GetEnableTracing() {
			mds = append(mds, kitTracing.Client())
		}
		if middlewareCfg.GetEnableMetadata() {
			mds = append(mds, metadata.Client())
		}
		authCfg := middlewareCfg.GetAuth()
		if authCfg != nil {
			authenticator, err := jwt.NewAuthenticator(
				jwt.WithKey([]byte(authCfg.GetSecret())),
				jwt.WithSigningMethod(authCfg.GetMethod()),
			)
			if err != nil {
				log.Error("create jwt authenticator failed", "method", authCfg.GetMethod(), "error", err)
			}
			mds = append(mds, authnMiddleware.Client(authenticator))
		}
		selectorFilterCfg := middlewareCfg.GetSelectorFilter()
		if selectorFilterCfg != nil {
			// 负载均衡过滤器
			if len(selectorFilterCfg.FilterVersion) != 0 {
				filter := selectorFilter.Version(selectorFilterCfg.FilterVersion)
				options = append(options, kratosHttp.WithNodeFilter(filter))
			}
			switch selectorFilterCfg.Balancer {
			case "p2c":
				selector.SetGlobalSelector(selectorP2c.NewBuilder())
			case "random":
				selector.SetGlobalSelector(selectorRandom.NewBuilder())
			case "wrr":
				selector.SetGlobalSelector(selectorWrr.NewBuilder())
			default:
				selector.SetGlobalSelector(selectorWrr.NewBuilder())
			}
		}
	}

	options = append(options, kratosHttp.WithMiddleware(mds...))

	if httpCfg.Tls != nil {
		var tlsCfg *tls.Config
		var err error

		if tlsCfg, err = utils.LoadClientTlsConfig(httpCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, kratosHttp.WithTLSConfig(tlsCfg))
		}
	}

	return options, nil
}

// CreateHttpServer 创建Http服务端
func CreateHttpServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosHttp.Server, error) {
	options, err := initHttpServerConfig(cfg, mds...)
	if err != nil {
		return nil, err
	}
	options = append(options, kratosHttp.ResponseEncoder(protoJSONResponseEncoder))

	srv := kratosHttp.NewServer(options...)

	if cfg != nil && cfg.Server != nil && cfg.Server.Http != nil && cfg.Server.Http.GetEnablePprof() {
		registerHttpPprof(srv)
	}

	return srv, nil
}

// initHttpServerConfig 初始化Http服务配置
func initHttpServerConfig(cfg *configv1.Bootstrap, mds ...middleware.Middleware) ([]kratosHttp.ServerOption, error) {
	if cfg == nil || cfg.Server == nil || cfg.Server.Http == nil {
		return nil, nil
	}

	httpCfg := cfg.Server.Http

	options := make([]kratosHttp.ServerOption, 0)

	if httpCfg.Cors != nil {
		options = append(options, kratosHttp.Filter(handlers.CORS(
			handlers.AllowedHeaders(httpCfg.Cors.Headers),
			handlers.AllowedMethods(httpCfg.Cors.Methods),
			handlers.AllowedOrigins(httpCfg.Cors.Origins),
		)))
	}

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := httpCfg.Middleware
	if middlewareCfg != nil {
		if middlewareCfg.GetEnableRecovery() {
			mds = append(mds, recovery.Recovery())
		}
		if middlewareCfg.GetEnableTracing() {
			mds = append(mds, kitTracing.Server())
		}
		if middlewareCfg.GetEnableValidate() {
			mds = append(mds, validate.ProtoValidate())
		}
		if middlewareCfg.GetEnableCircuitBreaker() {
		}
		if middlewareCfg.Limiter != nil {
			mds = append(mds, midRateLimit.Server())
		}
		if middlewareCfg.GetEnableMetadata() {
			mds = append(mds, metadata.Server())
		}
		mds = append(mds, requestid.NewRequestIDMiddleware())
	}

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

// registerHttpPprof 注册pprof路由
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
