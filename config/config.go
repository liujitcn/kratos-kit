package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/key"
	"github.com/liujitcn/kratos-kit/sdk"
)

const (
	remoteConfigSourceConfigFile = "config.yaml"
	keyConfigFile                = "key.yaml"
	rootKeyFile                  = "root.key"
)

// newFileConfigSource 创建一个本地文件配置源。
func newFileConfigSource(filePath string) config.Source {
	return file.NewSource(filePath)
}

// LoadBootstrapConfig 加载指定环境配置，并使用传入或运行时获取的密钥解密敏感字段。
func LoadBootstrapConfig(configPath, env string, keyValue key.Key) error {
	if keyValue == nil {
		keyValue = sdk.Runtime.GetKey()
	}
	if keyValue == nil {
		return fmt.Errorf("config: key is not initialized")
	}

	var err error
	var derived []byte
	derived, err = keyValue.Derive(context.Background(), "config")
	if err != nil {
		return fmt.Errorf("config: derive config key: %w", err)
	}
	var secret *SecretCipher
	secret, err = NewSecretCipher(derived)
	if err != nil {
		return err
	}
	var localSources []config.Source
	localSources, err = newEnvironmentFileConfigSources(configPath, env)
	if err != nil {
		return err
	}

	var remoteConfig *configv1.Config
	err, remoteConfig = loadRemoteConfigSourceConfigsWithDecoder(localSources, secret.Decoder())
	if err != nil {
		log.Error("loadRemoteConfigSourceConfigs: ", err.Error())
		return err
	}

	var cfg config.Config
	cfg, err = newConfigProviderWithDecoder(localSources, remoteConfig, secret.Decoder())
	if err != nil {
		return err
	}
	return loadBootstrapConfig(cfg)
}

// newEnvironmentFileConfigSources 按基础文件在前、环境覆盖文件在后的顺序创建本地配置源。
func newEnvironmentFileConfigSources(configPath, env string) ([]config.Source, error) {
	var err error
	err = validateEnvironment(env)
	if err != nil {
		return nil, err
	}

	var entries []os.DirEntry
	entries, err = os.ReadDir(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config path %q: %w", configPath, err)
	}

	baseSources := make([]config.Source, 0, len(entries))
	environmentSources := make([]config.Source, 0, len(entries))
	environmentSuffix := "." + env
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.Name() == rootKeyFile || entry.Name() == keyConfigFile || strings.HasPrefix(entry.Name(), "key.") {
			continue
		}

		extension := filepath.Ext(entry.Name())
		if extension == "" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), extension)
		filePath := filepath.Join(configPath, entry.Name())
		if !strings.Contains(name, ".") {
			baseSources = append(baseSources, newFileConfigSource(filePath))
			continue
		}
		if env != "" && strings.HasSuffix(name, environmentSuffix) && !strings.Contains(strings.TrimSuffix(name, environmentSuffix), ".") {
			environmentSources = append(environmentSources, newFileConfigSource(filePath))
		}
	}

	localSources := append(baseSources, environmentSources...)
	if len(localSources) == 0 {
		return nil, fmt.Errorf("no config files found in %q for environment %q", configPath, env)
	}
	return localSources, nil
}

// newConfigProviderWithDecoder 合并配置源，并按需安装敏感字段解码器。
func newConfigProviderWithDecoder(localSources []config.Source, remoteConfig *configv1.Config, decoder config.Decoder) (config.Config, error) {
	var err error
	if remoteConfig != nil {
		var remoteSource config.Source
		remoteSource, err = NewProvider(remoteConfig)
		if err != nil {
			log.Error("NewProvider: ", err.Error())
			return nil, err
		}
		localSources = append(localSources, remoteSource)
	}

	options := []config.Option{config.WithSource(localSources...)}
	if decoder != nil {
		options = append(options, config.WithDecoder(decoder))
	}
	return config.New(options...), nil
}

// loadBootstrapConfig 加载配置源并扫描所有已注册配置。
func loadBootstrapConfig(cfg config.Config) error {
	err := cfg.Load()
	if err != nil {
		return err
	}
	initBootstrapConfig()

	err = scanConfigs(cfg)
	if err != nil {
		return err
	}
	return nil
}

// scanConfigs 将配置源扫描到全部已注册的配置对象。
func scanConfigs(cfg config.Config) error {
	initBootstrapConfig()

	var err error
	for _, c := range configList {
		err = cfg.Scan(c)
		if err != nil {
			return err
		}
	}
	return nil
}

// loadRemoteConfigSourceConfigsWithDecoder 从本地配置源解析远程配置源参数，并按需解密敏感字段。
func loadRemoteConfigSourceConfigsWithDecoder(localSources []config.Source, decoder config.Decoder) (error, *configv1.Config) {
	options := []config.Option{config.WithSource(localSources...)}
	if decoder != nil {
		options = append(options, config.WithDecoder(decoder))
	}
	cfg := config.New(
		options...,
	)
	defer func(cfg config.Config) {
		closeErr := cfg.Close()
		if closeErr != nil {
			panic(closeErr)
		}
	}(cfg)

	err := cfg.Load()
	if err != nil {
		return err, nil
	}

	bootstrapConfig := &configv1.Bootstrap{}
	err = cfg.Scan(bootstrapConfig)
	if err != nil {
		return err, nil
	}

	return nil, bootstrapConfig.GetConfig()
}

// pathExists 判断路径是否存在
func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func validateEnvironment(env string) error {
	for index := 0; index < len(env); index++ {
		char := env[index]
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return fmt.Errorf("invalid runtime environment %q", env)
	}
	return nil
}
