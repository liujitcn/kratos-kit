package logger

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// CallerKey 表示日志调用位置字段名。
	CallerKey = "caller"
)

var ignoredLegacyFieldKeys = map[string]struct{}{
	"ts":               {},
	"service.id":       {},
	"service.instance": {},
	"service.version":  {},
}

// Field 描述一个按原始顺序保留的日志字段。
type Field struct {
	// Key 为字段名。
	Key string
	// Value 为字段原始值。
	Value any
}

// Entry 描述旧版 Kratos key/value 日志解析后的统一条目。
type Entry struct {
	// Message 为日志正文。
	Message string
	// Caller 为规范化后的“文件:行号”。
	Caller string
	// Fields 为过滤后按原始顺序保留的业务字段。
	Fields []Field
}

// ParseLegacyEntry 解析旧版 Kratos key/value 日志并过滤重复的标准字段。
func ParseLegacyEntry(keyvals ...any) (Entry, error) {
	var entry Entry
	var keylen = len(keyvals)
	if keylen == 0 {
		return entry, nil
	}
	if keylen%2 != 0 {
		return entry, fmt.Errorf("Keyvalues must appear in pairs: %v", keyvals)
	}

	entry.Fields = make([]Field, 0, keylen/2)
	for i := 0; i < keylen; i += 2 {
		var key = fmt.Sprint(keyvals[i])
		var value = keyvals[i+1]
		switch key {
		case slog.MessageKey:
			entry.Message = fmt.Sprint(value)
		case CallerKey:
			entry.Caller = ParseCaller(value)
		case "trace_id", "span_id":
			if isEmptyFieldValue(value) {
				continue
			}
			entry.Fields = append(entry.Fields, Field{Key: key, Value: value})
		default:
			if shouldIgnoreLegacyField(key) {
				continue
			}
			entry.Fields = append(entry.Fields, Field{Key: key, Value: value})
		}
	}

	// 兼容旧版 SQL 日志：消息前缀中的业务位置优先于日志包装层 caller。
	var inferredCaller = InferCallerFromMessage(entry.Message)
	if inferredCaller != "" {
		entry.Caller = inferredCaller
	}

	return entry, nil
}

// ParseCaller 将 caller 值规范化为斜杠分隔的“文件:行号”格式。
func ParseCaller(value any) string {
	var callerText = CleanANSI(strings.TrimSpace(fmt.Sprint(value)))
	var file string
	var line int
	var ok bool
	file, line, ok = splitCaller(callerText)
	if !ok {
		return ""
	}
	return file + ":" + strconv.Itoa(line)
}

// InferCallerFromMessage 尝试从日志消息首段提取“文件:行号”调用位置。
func InferCallerFromMessage(message string) string {
	if message == "" {
		return ""
	}

	var firstField = message
	var index = strings.IndexAny(message, " \t")
	if index >= 0 {
		firstField = message[:index]
	}
	var caller = ParseCaller(firstField)
	var file string
	var line int
	var ok bool
	file, line, ok = splitCaller(caller)
	if !ok || !strings.HasSuffix(file, ".go") {
		return ""
	}
	return file + ":" + strconv.Itoa(line)
}

// CleanANSI 移除文本中的 ANSI 控制码。
func CleanANSI(text string) string {
	if text == "" || !strings.Contains(text, "\x1b[") {
		return text
	}

	var builder strings.Builder
	builder.Grow(len(text))
	var inEscape bool
	var afterBracket bool
	for i := 0; i < len(text); i++ {
		var char = text[i]
		if inEscape {
			if !afterBracket {
				if char == '[' {
					afterBracket = true
				}
				continue
			}
			if char >= '@' && char <= '~' {
				inEscape = false
				afterBracket = false
			}
			continue
		}
		if char == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			inEscape = true
			continue
		}
		builder.WriteByte(char)
	}

	return builder.String()
}

// splitCaller 将“文件:行号”拆分为文件与行号。
func splitCaller(caller string) (string, int, bool) {
	var index = strings.LastIndexByte(caller, ':')
	if index <= 0 || index >= len(caller)-1 {
		return "", 0, false
	}

	var line, err = strconv.Atoi(caller[index+1:])
	if err != nil || line <= 0 {
		return "", 0, false
	}

	var file = strings.ReplaceAll(caller[:index], "\\", "/")
	file = filepath.ToSlash(file)
	if file == "" {
		return "", 0, false
	}
	return file, line, true
}

// shouldIgnoreLegacyField 判断是否忽略 provider 注入的重复标准字段。
func shouldIgnoreLegacyField(key string) bool {
	_, ok := ignoredLegacyFieldKeys[key]
	return ok
}

// isEmptyFieldValue 判断日志字段是否为空。
func isEmptyFieldValue(value any) bool {
	if value == nil {
		return true
	}
	return fmt.Sprint(value) == ""
}
