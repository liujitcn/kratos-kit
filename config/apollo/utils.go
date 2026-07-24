package apollo

import (
	"strings"

	"github.com/go-kratos/kratos/v3/log"
)

func format(ns string) string {
	arr := strings.Split(ns, ".")
	suffix := arr[len(arr)-1]
	if len(arr) <= 1 || suffix == properties {
		return json
	}
	if _, ok := formats[suffix]; !ok {
		// 未识别的 namespace 后缀按 JSON 处理，保持历史默认行为。
		return json
	}

	return suffix
}

// resolve 将点分隔的配置键展开到多层 map。
// 例如 app.name = "application" 会写入 map[app][name] = "application"。
func resolve(key string, value any, target map[string]any) {
	// 将 "aaa.bbb" 这类点分隔键展开为 map[aaa]map[bbb]interface{}。
	keys := strings.Split(key, ".")
	last := len(keys) - 1
	cursor := target

	for i, k := range keys {
		if i == last {
			cursor[k] = value
			break
		}

		// 当前不是最后一级键时，需要继续向更深层 map 写入。
		v, ok := cursor[k]
		if !ok {
			// 缺少中间层时主动创建，保证后续子键可以继续展开。
			deeper := make(map[string]any)
			cursor[k] = deeper
			cursor = deeper
			continue
		}

		// 已存在的中间层必须是 map，否则说明配置键存在冲突，无法继续展开。
		if cursor, ok = v.(map[string]any); !ok {
			log.Warn("duplicate key", "key", strings.Join(keys[:i+1], "."))
			break
		}
	}
}

// genKey 根据 namespace 与子键生成 config.KeyValue 使用的完整键名。
// 例如 namespace.ext 与 subKey 会生成 namespace.subKey。
func genKey(ns, sub string) string {
	arr := strings.Split(ns, ".")
	if len(arr) == 1 {
		if ns == "" {
			return sub
		}

		return ns + "." + sub
	}

	suffix := arr[len(arr)-1]
	_, ok := formats[suffix]
	if ok {
		return strings.Join(arr[:len(arr)-1], ".") + "." + sub
	}

	return ns + "." + sub
}
