package grpc

import (
	"crypto/tls"
	"fmt"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	"github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"
	"github.com/go-kratos/kratos/v3/transport/grpc"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/server/grpc/middleware/requestid"
	"github.com/liujitcn/kratos-kit/tracing"
	"github.com/liujitcn/kratos-kit/utils"
)

// CreateGrpcServer 创建 GRPC 服务端。
func CreateGrpcServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*grpc.Server, error) {
	options, err := initGrpcServerConfig(cfg, mds...)
	if err != nil {
		return nil, fmt.Errorf("init grpc server config failed: %w", err)
	}

	srv := grpc.NewServer(options...)

	return srv, nil
}

// initGrpcServerConfig 根据配置组装 Kratos gRPC 服务端选项。
func initGrpcServerConfig(cfg *configv1.Bootstrap, mds ...middleware.Middleware) ([]grpc.ServerOption, error) {
	if cfg == nil || cfg.Server == nil || cfg.Server.Grpc == nil {
		return nil, nil
	}

	grpcCfg := cfg.Server.Grpc

	options := make([]grpc.ServerOption, 0)

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := grpcCfg.Middleware
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

	options = append(options, grpc.Middleware(mds...))
	if grpcCfg.GetCustomHealth() {
		options = append(options, grpc.CustomHealth())
	}

	if grpcCfg.Tls != nil {
		var tlsCfg *tls.Config
		var err error

		if tlsCfg, err = utils.LoadServerTlsConfig(grpcCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, grpc.TLSConfig(tlsCfg))
		}
	}

	if grpcCfg.Network != "" {
		options = append(options, grpc.Network(grpcCfg.Network))
	}
	if grpcCfg.Addr != "" {
		options = append(options, grpc.Address(grpcCfg.Addr))
	}
	if grpcCfg.Timeout != nil {
		options = append(options, grpc.Timeout(grpcCfg.Timeout.AsDuration()))
	}

	return options, nil
}
