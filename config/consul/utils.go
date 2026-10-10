package consul

import "strings"

// getConfigKey 按配置选择器规则转换配置键。
func getConfigKey(configKey string, useBackslash bool) string {
	if useBackslash {
		return strings.ReplaceAll(configKey, `.`, `/`)
	}
	return configKey
}
