package rpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v3/selector"
	"github.com/liujitcn/kratos-kit/rpc/middleware/requestid"
	kitTracing "github.com/liujitcn/kratos-kit/tracing"
	"github.com/liujitcn/kratos-kit/utils"
	"google.golang.org/grpc"

	"github.com/go-kratos/kratos/v3/registry"

	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/metadata"
	midRateLimit "github.com/go-kratos/kratos/v3/middleware/ratelimit"
	"github.com/go-kratos/kratos/v3/middleware/recovery"

	kratosGrpc "github.com/go-kratos/kratos/v3/transport/grpc"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"

	selectorFilter "github.com/go-kratos/kratos/v3/selector/filter"
	selectorP2c "github.com/go-kratos/kratos/v3/selector/p2c"
	selectorRandom "github.com/go-kratos/kratos/v3/selector/random"
	selectorWrr "github.com/go-kratos/kratos/v3/selector/wrr"
)

const defaultTimeout = 5 * time.Second

// CreateGrpcClient 根据服务名或配置地址创建 gRPC 客户端。
func CreateGrpcClient(ctx context.Context, r registry.Discovery, serviceName string, cfg *configv1.Bootstrap, mds ...middleware.Middleware) (grpc.ClientConnInterface, error) {
	var err error
	var options []kratosGrpc.ClientOption

	var endpoint string
	endpoint, err = resolveGrpcClientEndpoint(serviceName, cfg)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(endpoint, "discovery:///") {
		if r == nil {
			return nil, fmt.Errorf("gRPC 客户端使用服务发现地址时 Discovery 不能为空: %s", endpoint)
		}
		options = append(options, kratosGrpc.WithDiscovery(r))
	}
	options = append(options, kratosGrpc.WithEndpoint(endpoint))

	options, err = initGrpcClientConfig(cfg, options, mds...)
	if err != nil {
		return nil, fmt.Errorf("init grpc client config failed: %w", err)
	}

	var conn grpc.ClientConnInterface
	conn, err = kratosGrpc.NewClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("dial grpc client [%s] failed: %w", endpoint, err)
	}

	return conn, nil
}

// resolveGrpcClientEndpoint 按显式服务名优先、配置地址回退的顺序解析客户端地址。
func resolveGrpcClientEndpoint(serviceName string, cfg *configv1.Bootstrap) (string, error) {
	if serviceName != "" {
		if strings.Contains(serviceName, "://") {
			return serviceName, nil
		}
		return "discovery:///" + serviceName, nil
	}
	if cfg == nil || cfg.GetClient() == nil || cfg.GetClient().GetGrpc() == nil {
		return "", fmt.Errorf("gRPC 客户端服务名和配置地址不能同时为空")
	}

	endpoint := cfg.GetClient().GetGrpc().GetEndpoint()
	if endpoint == "" {
		return "", fmt.Errorf("gRPC 客户端服务名和配置地址不能同时为空")
	}
	if strings.Contains(endpoint, "://") {
		return endpoint, nil
	}
	return "direct:///" + endpoint, nil
}

// initGrpcClientConfig 根据配置组装 Kratos gRPC 客户端选项。
func initGrpcClientConfig(cfg *configv1.Bootstrap, options []kratosGrpc.ClientOption, mds ...middleware.Middleware) ([]kratosGrpc.ClientOption, error) {
	if cfg == nil || cfg.Client == nil || cfg.Client.Grpc == nil {
		return options, nil
	}

	grpcCfg := cfg.Client.Grpc

	timeout := defaultTimeout
	if grpcCfg.Timeout != nil {
		timeout = grpcCfg.Timeout.AsDuration()
	}
	options = append(options, kratosGrpc.WithTimeout(timeout))

	middlewareCfg := grpcCfg.Middleware
	var err error
	mds, err = appendClientMiddlewares(middlewareCfg, mds)
	if err != nil {
		return nil, err
	}
	if middlewareCfg != nil {
		selectorFilterCfg := middlewareCfg.GetSelectorFilter()
		if selectorFilterCfg != nil {
			// 负载均衡过滤器
			if len(selectorFilterCfg.FilterVersion) != 0 {
				filter := selectorFilter.Version(selectorFilterCfg.FilterVersion)
				options = append(options, kratosGrpc.WithNodeFilter(filter))
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

	options = append(options, kratosGrpc.WithMiddleware(mds...))

	if grpcCfg.Tls != nil {
		var tlsCfg *tls.Config

		if tlsCfg, err = utils.LoadClientTlsConfig(grpcCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, kratosGrpc.WithTLSConfig(tlsCfg))
		}
	}

	return options, nil
}

// CreateGrpcServer 创建GRPC服务端
func CreateGrpcServer(cfg *configv1.Bootstrap, mds ...middleware.Middleware) (*kratosGrpc.Server, error) {
	options, err := initGrpcServerConfig(cfg, mds...)
	if err != nil {
		return nil, fmt.Errorf("init grpc server config failed: %w", err)
	}

	srv := kratosGrpc.NewServer(options...)

	return srv, nil
}

// initGrpcServerConfig 根据配置组装 Kratos gRPC 服务端选项。
func initGrpcServerConfig(cfg *configv1.Bootstrap, mds ...middleware.Middleware) ([]kratosGrpc.ServerOption, error) {
	if cfg == nil || cfg.Server == nil || cfg.Server.Grpc == nil {
		return nil, nil
	}

	grpcCfg := cfg.Server.Grpc

	options := make([]kratosGrpc.ServerOption, 0)

	if mds == nil {
		mds = make([]middleware.Middleware, 0)
	}

	middlewareCfg := grpcCfg.Middleware

	if middlewareCfg != nil {
		mds = append([]middleware.Middleware{requestid.Server()}, mds...)
		if middlewareCfg.GetEnableRecovery() {
			mds = append(mds, recovery.Recovery())
		}
		if middlewareCfg.GetEnableTracing() {
			mds = append(mds, kitTracing.Server())
		}
		if middlewareCfg.GetEnableValidate() {
			mds = append(mds, protoValidateMiddleware())
		}
		if middlewareCfg.Limiter != nil {
			mds = append(mds, midRateLimit.Server())
		}
		if middlewareCfg.GetEnableMetadata() {
			mds = append(mds, metadata.Server())
		}
	}

	options = append(options, kratosGrpc.Middleware(mds...))
	if grpcCfg.GetCustomHealth() {
		options = append(options, kratosGrpc.CustomHealth())
	}

	if grpcCfg.Tls != nil {
		var tlsCfg *tls.Config
		var err error

		if tlsCfg, err = utils.LoadServerTlsConfig(grpcCfg.Tls); err != nil {
			return nil, err
		}

		if tlsCfg != nil {
			options = append(options, kratosGrpc.TLSConfig(tlsCfg))
		}
	}

	if grpcCfg.Network != "" {
		options = append(options, kratosGrpc.Network(grpcCfg.Network))
	}
	if grpcCfg.Addr != "" {
		options = append(options, kratosGrpc.Address(grpcCfg.Addr))
	}
	if grpcCfg.Timeout != nil {
		options = append(options, kratosGrpc.Timeout(grpcCfg.Timeout.AsDuration()))
	}

	return options, nil
}
