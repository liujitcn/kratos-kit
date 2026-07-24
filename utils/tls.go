package utils

import (
	"crypto/tls"

	_tls "github.com/liujitcn/go-utils/tls"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// LoadServerTlsConfig 根据配置加载服务端 TLS 配置。
func LoadServerTlsConfig(cfg *configv1.Tls) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	var tlsCfg *tls.Config
	var err error

	// 优先按证书文件路径加载 TLS 配置。
	if cfg.File != nil {
		if tlsCfg, err = _tls.LoadServerTlsConfigFile(
			cfg.File.GetKeyPath(),
			cfg.File.GetCertPath(),
			cfg.File.GetCaPath(),
			cfg.InsecureSkipVerify,
		); err != nil {
			return nil, err
		}
	} else if cfg.Config != nil {
		// 未配置文件路径时，允许直接使用内存中的 PEM 内容。
		if tlsCfg, err = _tls.LoadServerTlsConfigString(
			cfg.Config.GetKeyPem(),
			cfg.Config.GetCertPem(),
			cfg.Config.GetCaPem(),
			cfg.InsecureSkipVerify,
		); err != nil {
			return nil, err
		}
	}

	return tlsCfg, err
}

// LoadClientTlsConfig 根据配置加载客户端 TLS 配置。
func LoadClientTlsConfig(cfg *configv1.Tls) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	var tlsCfg *tls.Config
	var err error

	// 优先按证书文件路径加载 TLS 配置。
	if cfg.File != nil {
		if tlsCfg, err = _tls.LoadClientTlsConfigFile(
			cfg.File.GetKeyPath(),
			cfg.File.GetCertPath(),
			cfg.File.GetCaPath(),
		); err != nil {
			return nil, err
		}
	} else if cfg.Config != nil {
		// 未配置文件路径时，允许直接使用内存中的 PEM 内容。
		if tlsCfg, err = _tls.LoadClientTlsConfigString(
			cfg.Config.GetKeyPem(),
			cfg.Config.GetCertPem(),
			cfg.Config.GetCaPem(),
		); err != nil {
			return nil, err
		}
	}

	return tlsCfg, err
}
