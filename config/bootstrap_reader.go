package config

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	kratosconfig "github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/encoding"
	"github.com/go-kratos/kratos/v3/log"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

var bootstrapConfigPlaceholderPattern = regexp.MustCompile(`\${(.*?)}`)

// loadBootstrapConfigWithoutWatch 读取并合并配置源，但不为临时配置源启动 watcher。
func loadBootstrapConfigWithoutWatch(sources []kratosconfig.Source, decoder kratosconfig.Decoder) (*configv1.Bootstrap, error) {
	bootstrapConfig := &configv1.Bootstrap{}
	err := LoadConfigWithoutWatch(sources, decoder, bootstrapConfig)
	if err != nil {
		return nil, err
	}
	return bootstrapConfig, nil
}

// LoadConfigWithoutWatch 读取并合并配置源一次，不启动配置 watcher。
func LoadConfigWithoutWatch(sources []kratosconfig.Source, decoder kratosconfig.Decoder, target any) error {
	values, err := loadConfigValuesWithoutWatch(sources, decoder)
	if err != nil {
		return err
	}
	data, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("config: marshal bootstrap config: %w", err)
	}
	if message, ok := target.(proto.Message); ok {
		err = protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, message)
	} else {
		err = json.Unmarshal(data, target)
	}
	if err != nil {
		return fmt.Errorf("config: unmarshal bootstrap config: %w", err)
	}
	return nil
}

// loadConfigValuesWithoutWatch 读取并合并配置源的原始值，不启动配置 watcher。
func loadConfigValuesWithoutWatch(sources []kratosconfig.Source, decoder kratosconfig.Decoder) (map[string]any, error) {
	values := make(map[string]any)
	var err error
	for _, source := range sources {
		var keyValues []*kratosconfig.KeyValue
		keyValues, err = source.Load()
		if err != nil {
			return nil, err
		}
		for _, keyValue := range keyValues {
			decoded := make(map[string]any)
			if decoder == nil {
				err = decodeBootstrapKeyValue(keyValue, decoded)
			} else {
				err = decoder(keyValue, decoded)
			}
			if err != nil {
				log.Error("failed to decode bootstrap config", "error", err, "key", keyValue.Key)
				return nil, err
			}
			mergeBootstrapConfigValues(values, decoded)
		}
	}

	resolveBootstrapConfigValues(values)
	return values, nil
}

// decodeBootstrapKeyValue 使用 Kratos 默认规则解码不带敏感字段处理器的配置项。
func decodeBootstrapKeyValue(src *kratosconfig.KeyValue, target map[string]any) error {
	if src == nil {
		return fmt.Errorf("config: key value is nil")
	}
	if src.Format == "" {
		keys := strings.Split(src.Key, ".")
		var current = target
		for index, key := range keys {
			if index == len(keys)-1 {
				current[key] = src.Value
				return nil
			}
			sub := make(map[string]any)
			current[key] = sub
			current = sub
		}
		return nil
	}
	codec := encoding.GetCodec(strings.ToLower(src.Format))
	if codec == nil {
		return fmt.Errorf("config: unsupported key: %s format: %s", src.Key, src.Format)
	}
	return codec.Unmarshal(src.Value, &target)
}

// mergeBootstrapConfigValues 按 Kratos 配置规则递归合并配置值。
func mergeBootstrapConfigValues(dst, src map[string]any) {
	for key, sourceValue := range src {
		normalizedValue := normalizeBootstrapConfigValue(sourceValue)
		sourceMap, ok := normalizedValue.(map[string]any)
		if !ok {
			dst[key] = normalizedValue
			continue
		}

		destinationMap, ok := normalizeBootstrapConfigValue(dst[key]).(map[string]any)
		if !ok {
			destinationMap = make(map[string]any)
		}
		mergeBootstrapConfigValues(destinationMap, sourceMap)
		dst[key] = destinationMap
	}
}

// normalizeBootstrapConfigValue 将配置树中的 map 和 slice 统一为可合并的类型。
func normalizeBootstrapConfigValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized[key] = normalizeBootstrapConfigValue(child)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized[fmt.Sprint(key)] = normalizeBootstrapConfigValue(child)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for index, child := range typed {
			normalized[index] = normalizeBootstrapConfigValue(child)
		}
		return normalized
	default:
		return value
	}
}

// resolveBootstrapConfigValues 解析配置中的 ${key:default} 占位符。
func resolveBootstrapConfigValues(values map[string]any) {
	for key, value := range values {
		values[key] = resolveBootstrapConfigValue(value, values)
	}
}

// resolveBootstrapConfigValue 递归解析配置树中的字符串占位符。
func resolveBootstrapConfigValue(value any, values map[string]any) any {
	switch typed := value.(type) {
	case string:
		return expandBootstrapConfigValue(typed, values)
	case map[string]any:
		for key, child := range typed {
			typed[key] = resolveBootstrapConfigValue(child, values)
		}
		return typed
	case []any:
		for index, child := range typed {
			typed[index] = resolveBootstrapConfigValue(child, values)
		}
		return typed
	default:
		return value
	}
}

// expandBootstrapConfigValue 替换单个配置字符串中的占位符。
func expandBootstrapConfigValue(value string, values map[string]any) string {
	matches := bootstrapConfigPlaceholderPattern.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) != 2 { //nolint:mnd
			continue
		}
		args := strings.SplitN(strings.TrimSpace(match[1]), ":", 2) //nolint:mnd
		replacement := ""
		if referencedValue, ok := lookupBootstrapConfigValue(values, args[0]); ok {
			replacement, _ = stringifyBootstrapConfigValue(referencedValue)
		} else if len(args) == 2 { //nolint:mnd
			replacement = args[1]
		}
		value = strings.ReplaceAll(value, match[0], replacement)
	}
	return value
}

// lookupBootstrapConfigValue 按点号路径查找配置值。
func lookupBootstrapConfigValue(values map[string]any, path string) (any, bool) {
	var current any = values
	for _, key := range strings.Split(path, ".") {
		currentMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = currentMap[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// stringifyBootstrapConfigValue 将占位符引用的基础类型转换为字符串。
func stringifyBootstrapConfigValue(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(typed), true
	case []byte:
		return string(typed), true
	case fmt.Stringer:
		return typed.String(), true
	default:
		return "", false
	}
}
