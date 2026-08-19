package logger

import (
	"encoding/json"
	"strings"
)

// FormatFields 将结构化字段编码为与 zap 控制台一致的单行 JSON。
func FormatFields(fields map[string]any) (string, error) {
	var encoded, err = json.Marshal(fields)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.Grow(len(encoded) + len(fields)*2)
	var inString bool
	var escaped bool
	for _, char := range encoded {
		builder.WriteByte(char)
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if char == '\\' {
				escaped = true
				continue
			}
			if char == '"' {
				inString = false
			}
			continue
		}
		if char == '"' {
			inString = true
			continue
		}
		if char == ':' || char == ',' {
			builder.WriteByte(' ')
		}
	}
	return builder.String(), nil
}
