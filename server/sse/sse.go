package sse

import (
	"errors"
	"fmt"
	"net/http"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/transport/sse"
	"github.com/liujitcn/kratos-kit/utils"
)

// CreateSseServer 创建独立监听端口的 SSE 服务端。
func CreateSseServer(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (*sse.Server, error) {
	err := validateSseServerTransport(cfg, configv1.Server_Sse_HTTP)
	if err != nil {
		return nil, err
	}

	options, err := initSseServerConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

	return sse.NewServer(options...)
}

// CreateSseHandler 创建可挂载到已有 HTTP 服务的 SSE 处理器。
func CreateSseHandler(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (*sse.Server, error) {
	err := validateSseServerTransport(cfg, configv1.Server_Sse_IN_PROCESS)
	if err != nil {
		return nil, err
	}

	options, err := initSseServerConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

	return sse.NewHandler(options...), nil
}

// CreateSseHTTPHandler 创建标准 http.Handler 形式的 SSE 处理器。
func CreateSseHTTPHandler(cfg *configv1.Bootstrap, opts ...sse.ServerOption) (http.Handler, error) {
	return CreateSseHandler(cfg, opts...)
}

// validateSseServerTransport 校验 server.sse.transport 与当前构造函数匹配。
func validateSseServerTransport(cfg *configv1.Bootstrap, expected configv1.Server_Sse_Transport) error {
	if cfg == nil || cfg.Server == nil || cfg.Server.Sse == nil {
		return nil
	}

	switch cfg.Server.Sse.GetTransport() {
	case configv1.Server_Sse_UNSPECIFIED, expected:
		return nil
	case configv1.Server_Sse_HTTP:
		return errors.New("server.sse.transport=HTTP requires CreateSseServer")
	case configv1.Server_Sse_IN_PROCESS:
		return errors.New("server.sse.transport=IN_PROCESS requires CreateSseHandler or CreateSseHTTPHandler")
	default:
		return fmt.Errorf("unsupported sse server transport: %s", cfg.Server.Sse.GetTransport())
	}
}

// initSseServerConfig 初始化 SSE 服务端配置。
func initSseServerConfig(cfg *configv1.Bootstrap, opts ...sse.ServerOption) ([]sse.ServerOption, error) {
	options := make([]sse.ServerOption, 0, len(opts)+12)

	if cfg == nil || cfg.Server == nil || cfg.Server.Sse == nil {
		return append(options, opts...), nil
	}

	sseCfg := cfg.Server.Sse
	if sseCfg.Network != "" {
		options = append(options, sse.WithNetwork(sseCfg.Network))
	}
	if sseCfg.Addr != "" {
		options = append(options, sse.WithAddress(sseCfg.Addr))
	}
	if sseCfg.Path != "" {
		options = append(options, sse.WithPath(sseCfg.Path))
	}
	if sseCfg.Codec != "" {
		options = append(options, sse.WithCodec(sseCfg.Codec))
	}
	if sseCfg.Timeout != nil {
		options = append(options, sse.WithTimeout(sseCfg.Timeout.AsDuration()))
	}
	if sseCfg.EventTtl != nil {
		options = append(options, sse.WithEventTTL(sseCfg.EventTtl.AsDuration()))
	}

	options = append(options,
		sse.WithAutoStream(sseCfg.GetAutoStream()),
		sse.WithAutoReply(sseCfg.GetAutoReply()),
		sse.WithSplitData(sseCfg.GetSplitData()),
		sse.WithEncodeBase64(sseCfg.GetEncodeBase64()),
	)

	if sseCfg.Tls != nil {
		tlsCfg, err := utils.LoadServerTlsConfig(sseCfg.Tls)
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			options = append(options, sse.WithTLSConfig(tlsCfg))
		}
	}

	return append(options, opts...), nil
}
