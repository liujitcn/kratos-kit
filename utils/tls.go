package utils

import (
	"crypto/tls"

	_tls "github.com/liujitcn/go-utils/tls"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
)

func LoadServerTlsConfig(cfg *conf.TLS) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	var tlsCfg *tls.Config
	var err error

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

func LoadClientTlsConfig(cfg *conf.TLS) (*tls.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	var tlsCfg *tls.Config
	var err error

	if cfg.File != nil {
		if tlsCfg, err = _tls.LoadClientTlsConfigFile(
			cfg.File.GetKeyPath(),
			cfg.File.GetCertPath(),
			cfg.File.GetCaPath(),
		); err != nil {
			return nil, err
		}
	} else if cfg.Config != nil {
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
