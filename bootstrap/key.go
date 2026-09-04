package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	kratosconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	kitconfig "github.com/liujitcn/kratos-kit/config"
)

const keyConfigFile = "key.yaml"

// loadKeyConfigWithEnv 从独立的 key.yaml 读取密钥 Provider 描述。
// 该方法只读取本地 key 文件，不加载远程配置，也不会尝试解密字段。
func loadKeyConfigWithEnv(configPath, env string) (*configv1.Key, error) {
	var err error
	err = validateKeyEnvironment(env)
	if err != nil {
		return nil, err
	}

	sources := make([]kratosconfig.Source, 0, 2)
	basePath := filepath.Join(configPath, keyConfigFile)
	if keyPathExists(basePath) {
		sources = append(sources, file.NewSource(basePath))
	}
	if env != "" {
		envPath := filepath.Join(configPath, "key."+env+".yaml")
		if keyPathExists(envPath) {
			sources = append(sources, file.NewSource(envPath))
		}
	}
	if len(sources) == 0 {
		return nil, nil
	}

	keyConfig := &configv1.Key{}
	err = kitconfig.LoadConfigWithoutWatch(sources, nil, keyConfig)
	if err != nil {
		return nil, err
	}
	return keyConfig, nil
}

// keyPathExists 判断密钥配置路径是否存在。
func keyPathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// validateKeyEnvironment 校验密钥配置使用的环境名称。
func validateKeyEnvironment(env string) error {
	for index := 0; index < len(env); index++ {
		char := env[index]
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '-' || char == '_' {
			continue
		}
		return fmt.Errorf("invalid runtime environment %q", env)
	}
	return nil
}
