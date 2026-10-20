package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/broker"
)

// NewServerFromConfig 根据公共 server.mqtt 配置创建 MQTT 订阅服务。
func NewServerFromConfig(cfg *configv1.Server_Mqtt) (*Server, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.GetEndpoint() == "" {
		return nil, errors.New("mqtt: server.mqtt.endpoint is required when configured")
	}
	options := []ServerOption{WithAddress(cfg.GetEndpoint())}
	if cfg.GetCodec() != "" {
		options = append(options, WithCodec(cfg.GetCodec()))
	}
	if cfg.GetClientId() != "" {
		options = append(options, WithClientID(cfg.GetClientId()))
	}
	if cfg.GetUsername() != "" || cfg.GetPassword() != "" {
		options = append(options, WithAuth(cfg.GetUsername(), cfg.GetPassword()))
	}
	if cfg.CleanSession != nil {
		options = append(options, WithCleanSession(cfg.GetCleanSession()))
	}
	if cfg.AutoReconnect != nil {
		options = append(options, WithAutoReconnect(cfg.GetAutoReconnect()))
	}
	if cfg.ResumeSubs != nil {
		options = append(options, WithResumeSubs(cfg.GetResumeSubs()))
	}
	if cfg.OrderMatters != nil {
		options = append(options, WithOrderMatters(cfg.GetOrderMatters()))
	}
	if cfg.GetKeepAlive() != nil {
		options = append(options, WithKeepAlive(cfg.GetKeepAlive().AsDuration()))
	}
	if cfg.GetWriteTimeout() != nil {
		options = append(options, WithWriteTimeout(cfg.GetWriteTimeout().AsDuration()))
	}
	if cfg.GetConnectRetryInterval() != nil {
		options = append(options, WithConnectRetryInterval(cfg.GetConnectRetryInterval().AsDuration()))
	}
	if cfg.GetMaxReconnectInterval() != nil {
		options = append(options, WithMaxReconnectInterval(cfg.GetMaxReconnectInterval().AsDuration()))
	}
	if cfg.GetTls() != nil {
		tlsConfig, err := loadClientTLSConfig(cfg.GetTls())
		if err != nil {
			return nil, err
		}
		if tlsConfig != nil {
			options = append(options, WithBrokerOptions(broker.WithTLSConfig(tlsConfig)))
		}
	}
	return NewServer(options...), nil
}

func loadClientTLSConfig(cfg *configv1.Tls) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}
	result := &tls.Config{InsecureSkipVerify: cfg.GetInsecureSkipVerify(), ServerName: cfg.GetDomain()} //nolint:gosec // 配置显式允许时才跳过校验。
	if file := cfg.GetFile(); file != nil {
		certPath := file.GetCertPath()
		keyPath := file.GetKeyPath()
		if certPath != "" || keyPath != "" {
			if certPath == "" || keyPath == "" {
				return nil, errors.New("mqtt: tls cert and key must be configured together")
			}
			certPEM, err := os.ReadFile(certPath)
			if err != nil {
				return nil, fmt.Errorf("mqtt: read tls cert: %w", err)
			}
			keyPEM, err := os.ReadFile(keyPath)
			if err != nil {
				return nil, fmt.Errorf("mqtt: read tls key: %w", err)
			}
			pair, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("mqtt: parse tls key pair: %w", err)
			}
			result.Certificates = []tls.Certificate{pair}
		}
		if file.GetCaPath() != "" {
			caPEM, err := os.ReadFile(file.GetCaPath())
			if err != nil {
				return nil, fmt.Errorf("mqtt: read tls ca: %w", err)
			}
			result.RootCAs, err = certificatePool(caPEM)
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	if config := cfg.GetConfig(); config != nil {
		var err error
		certPEM := config.GetCertPem()
		keyPEM := config.GetKeyPem()
		if len(certPEM) > 0 || len(keyPEM) > 0 {
			if len(certPEM) == 0 || len(keyPEM) == 0 {
				return nil, errors.New("mqtt: tls cert and key must be configured together")
			}
			pair, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return nil, fmt.Errorf("mqtt: parse tls key pair: %w", err)
			}
			result.Certificates = []tls.Certificate{pair}
		}
		if len(config.GetCaPem()) > 0 {
			result.RootCAs, err = certificatePool(config.GetCaPem())
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func certificatePool(pemBytes []byte) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(pemBytes) {
		return nil, errors.New("mqtt: parse tls ca failed")
	}
	return pool, nil
}
