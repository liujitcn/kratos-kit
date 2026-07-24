package rpc

import (
	"errors"
	"fmt"
	"net/http"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	sseServer "github.com/liujitcn/kratos-kit/transport/sse"
	"github.com/liujitcn/kratos-kit/utils"
)

// CreateSseServer 创建独立监听端口的 SSE 服务端。
func CreateSseServer(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (*sseServer.Server, error) {
	err := validateSseServerTransport(cfg, configv1.Server_Sse_HTTP)
	if err != nil {
		return nil, err
	}

	options, err := initSseServerConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

	return sseServer.NewServer(options...), nil
}

// CreateSseHandler 创建可挂载到已有 HTTP 服务的 SSE 处理器。
func CreateSseHandler(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (*sseServer.Server, error) {
	err := validateSseServerTransport(cfg, configv1.Server_Sse_IN_PROCESS)
	if err != nil {
		return nil, err
	}

	options, err := initSseServerConfig(cfg, opts...)
	if err != nil {
		return nil, err
	}

	return sseServer.NewHandler(options...), nil
}

// CreateSseHTTPHandler 创建标准 http.Handler 形式的 SSE 处理器。
func CreateSseHTTPHandler(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) (http.Handler, error) {
	return CreateSseHandler(cfg, opts...)
}

// CreateSseClient 创建 SSE 客户端。
func CreateSseClient(endpoint string, opts ...sseServer.ClientOption) *sseServer.Client {
	clientOptions := make([]func(*sseServer.Client), 0, len(opts))
	for _, opt := range opts {
		clientOptions = append(clientOptions, opt)
	}

	return sseServer.NewClient(endpoint, clientOptions...)
}

// CreateSseClientWithConfig 根据 client.sse 配置创建 SSE 客户端。
func CreateSseClientWithConfig(cfg *configv1.Bootstrap, opts ...sseServer.ClientOption) (*sseServer.Client, error) {
	endpoint := ""
	if cfg != nil && cfg.Client != nil && cfg.Client.Sse != nil {
		endpoint = cfg.Client.Sse.GetEndpoint()
	}

	client := CreateSseClient(endpoint, opts...)
	if cfg == nil || cfg.Client == nil || cfg.Client.Sse == nil {
		return client, nil
	}

	sseCfg := cfg.Client.Sse
	if sseCfg.Timeout != nil {
		client.Connection.Timeout = sseCfg.Timeout.AsDuration()
	}
	for key, val := range sseCfg.GetMetadata() {
		client.Headers[key] = val
	}
	client.EncodingBase64 = sseCfg.GetEncodeBase64()
	if sseCfg.Tls != nil {
		tlsCfg, err := utils.LoadClientTlsConfig(sseCfg.Tls)
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			client.Connection.Transport = &http.Transport{
				TLSClientConfig: tlsCfg,
			}
		}
	}

	return client, nil
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
func initSseServerConfig(cfg *configv1.Bootstrap, opts ...sseServer.ServerOption) ([]sseServer.ServerOption, error) {
	options := make([]sseServer.ServerOption, 0, len(opts)+12)

	if cfg == nil || cfg.Server == nil || cfg.Server.Sse == nil {
		return append(options, opts...), nil
	}

	sseCfg := cfg.Server.Sse
	if sseCfg.Network != "" {
		options = append(options, sseServer.WithNetwork(sseCfg.Network))
	}
	if sseCfg.Addr != "" {
		options = append(options, sseServer.WithAddress(sseCfg.Addr))
	}
	if sseCfg.Path != "" {
		options = append(options, sseServer.WithPath(sseCfg.Path))
	}
	if sseCfg.Codec != "" {
		options = append(options, sseServer.WithCodec(sseCfg.Codec))
	}
	if sseCfg.Timeout != nil {
		options = append(options, sseServer.WithTimeout(sseCfg.Timeout.AsDuration()))
	}
	if sseCfg.EventTtl != nil {
		options = append(options, sseServer.WithEventTTL(sseCfg.EventTtl.AsDuration()))
	}

	options = append(options,
		sseServer.WithAutoStream(sseCfg.GetAutoStream()),
		sseServer.WithAutoReply(sseCfg.GetAutoReply()),
		sseServer.WithSplitData(sseCfg.GetSplitData()),
		sseServer.WithEncodeBase64(sseCfg.GetEncodeBase64()),
	)

	if sseCfg.Tls != nil {
		tlsCfg, err := utils.LoadServerTlsConfig(sseCfg.Tls)
		if err != nil {
			return nil, err
		}
		if tlsCfg != nil {
			options = append(options, sseServer.WithTLSConfig(tlsCfg))
		}
	}

	return append(options, opts...), nil
}
