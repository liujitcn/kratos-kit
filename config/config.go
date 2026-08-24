package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v3/config"
	fileKratos "github.com/go-kratos/kratos/v3/config/file"
	"github.com/go-kratos/kratos/v3/log"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

const remoteConfigSourceConfigFile = "config.yaml"

// NewFileConfigSource 创建一个本地文件配置源
func NewFileConfigSource(filePath string) config.Source {
	return fileKratos.NewSource(filePath)
}

// NewConfigProvider 创建加载完整目录的配置，保留原有加载行为。
func NewConfigProvider(configPath string) (config.Config, error) {
	err, remoteConfig := LoadRemoteConfigSourceConfigs(configPath)
	if err != nil {
		log.Error("LoadRemoteConfigSourceConfigs: ", err.Error())
		return nil, err
	}

	return newConfigProvider([]config.Source{NewFileConfigSource(configPath)}, remoteConfig)
}

// NewConfigProviderWithEnv 创建加载基础文件和指定环境覆盖文件的配置。
func NewConfigProviderWithEnv(configPath, env string) (config.Config, error) {
	localSources, err := newEnvironmentFileConfigSources(configPath, env)
	if err != nil {
		return nil, err
	}

	var remoteConfig *configv1.Config
	err, remoteConfig = loadRemoteConfigSourceConfigs(localSources)
	if err != nil {
		log.Error("loadRemoteConfigSourceConfigs: ", err.Error())
		return nil, err
	}

	return newConfigProvider(localSources, remoteConfig)
}

// newEnvironmentFileConfigSources 按基础文件在前、环境覆盖文件在后的顺序创建本地配置源。
func newEnvironmentFileConfigSources(configPath, env string) ([]config.Source, error) {
	for index := 0; index < len(env); index++ {
		char := env[index]
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return nil, fmt.Errorf("invalid runtime environment %q", env)
	}

	entries, err := os.ReadDir(configPath)
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

		extension := filepath.Ext(entry.Name())
		if extension == "" {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), extension)
		filePath := filepath.Join(configPath, entry.Name())
		if !strings.Contains(name, ".") {
			baseSources = append(baseSources, NewFileConfigSource(filePath))
			continue
		}
		if env != "" && strings.HasSuffix(name, environmentSuffix) && !strings.Contains(strings.TrimSuffix(name, environmentSuffix), ".") {
			environmentSources = append(environmentSources, NewFileConfigSource(filePath))
		}
	}

	localSources := append(baseSources, environmentSources...)
	if len(localSources) == 0 {
		return nil, fmt.Errorf("no config files found in %q for environment %q", configPath, env)
	}
	return localSources, nil
}

// newConfigProvider 合并本地配置源与可选的远程配置源。
func newConfigProvider(localSources []config.Source, remoteConfig *configv1.Config) (config.Config, error) {
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

	return config.New(
		config.WithSource(localSources...),
	), nil
}

// LoadBootstrapConfig 加载完整目录中的程序引导配置，保留原有加载行为。
func LoadBootstrapConfig(configPath string) error {
	cfg, err := NewConfigProvider(configPath)
	if err != nil {
		return err
	}
	return loadBootstrapConfig(cfg)
}

// LoadBootstrapConfigWithEnv 加载基础文件和指定环境覆盖文件中的程序引导配置。
func LoadBootstrapConfigWithEnv(configPath, env string) error {
	cfg, err := NewConfigProviderWithEnv(configPath, env)
	if err != nil {
		return err
	}
	return loadBootstrapConfig(cfg)
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

// LoadRemoteConfigSourceConfigs 加载远程配置源的本地配置
func LoadRemoteConfigSourceConfigs(configPath string) (error, *configv1.Config) {
	configPath = filepath.Join(configPath, remoteConfigSourceConfigFile)
	if !pathExists(configPath) {
		return nil, nil
	}
	return loadRemoteConfigSourceConfigs([]config.Source{NewFileConfigSource(configPath)})
}

// loadRemoteConfigSourceConfigs 从已筛选的本地配置源解析远程配置源参数。
func loadRemoteConfigSourceConfigs(localSources []config.Source) (error, *configv1.Config) {
	cfg := config.New(
		config.WithSource(localSources...),
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
