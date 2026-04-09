package zap

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/go-kratos/kratos/v2/log"
)

var _ log.Logger = (*Logger)(nil)

var ignoredFieldKeys = map[string]struct{}{
	"ts":               {},
	"service.id":       {},
	"service.instance": {},
	"service.version":  {},
}

type Logger struct {
	core   zapcore.Core
	msgKey string
}

// NewZapLogger 基于 zap core 创建 Kratos 日志适配器。
func NewZapLogger(core zapcore.Core) *Logger {
	return &Logger{
		core:   core,
		msgKey: log.DefaultMessageKey,
	}
}

// Log 将 Kratos 的 keyvals 转换为 zap entry，优先使用业务 caller 作为主输出位置。
func (l *Logger) Log(level log.Level, keyvals ...any) error {
	var zapLevel = toZapLevel(level)
	if zapLevel < zapcore.DPanicLevel && !l.core.Enabled(zapLevel) {
		return nil
	}

	var msg string
	var fields []zapcore.Field
	var caller zapcore.EntryCaller
	var err error

	msg, fields, caller, err = l.parseKeyvals(keyvals)
	if err != nil {
		return l.writeEntry(zapcore.WarnLevel, err.Error(), nil, zapcore.EntryCaller{})
	}

	return l.writeEntry(zapLevel, msg, fields, caller)
}

// parseKeyvals 解析 Kratos 传入的 keyvals，并过滤掉重复的标准字段。
func (l *Logger) parseKeyvals(keyvals []any) (string, []zapcore.Field, zapcore.EntryCaller, error) {
	var keylen = len(keyvals)
	if keylen == 0 {
		return "", nil, zapcore.EntryCaller{}, nil
	}
	if keylen%2 != 0 {
		return "", nil, zapcore.EntryCaller{}, fmt.Errorf("Keyvalues must appear in pairs: %v", keyvals)
	}

	var msg string
	var caller zapcore.EntryCaller
	var fields = make([]zapcore.Field, 0, keylen/2)

	for i := 0; i < keylen; i += 2 {
		var key = fmt.Sprint(keyvals[i])
		var value = keyvals[i+1]

		switch key {
		case l.msgKey:
			msg = fmt.Sprint(value)
		case "caller":
			caller = parseCaller(value)
		case "trace_id", "span_id":
			// 空 trace/span 没有信息量，直接跳过，避免日志尾部噪声。
			if isEmptyFieldValue(value) {
				continue
			}
			fields = append(fields, zap.String(key, fmt.Sprint(value)))
		default:
			if shouldIgnoreField(key) {
				continue
			}
			fields = append(fields, zap.Any(key, value))
		}
	}

	// SQL 日志会把真实调用位置放在消息前缀里，这里优先提取出来覆盖包装层 caller。
	if inferredCaller, ok := inferCallerFromMessage(msg); ok {
		caller = inferredCaller
	}

	return msg, fields, caller, nil
}

// writeEntry 直接向 zap core 写入 entry，避免包装层覆盖真实 caller。
func (l *Logger) writeEntry(level zapcore.Level, msg string, fields []zapcore.Field, caller zapcore.EntryCaller) error {
	var entry = zapcore.Entry{
		Level:   level,
		Time:    time.Now(),
		Message: msg,
		Caller:  caller,
	}

	return l.core.Write(entry, fields)
}

// toZapLevel 将 Kratos 日志级别映射到 zap，避免直接强转导致 fatal 级别错位。
func toZapLevel(level log.Level) zapcore.Level {
	switch level {
	case log.LevelDebug:
		return zapcore.DebugLevel
	case log.LevelInfo:
		return zapcore.InfoLevel
	case log.LevelWarn:
		return zapcore.WarnLevel
	case log.LevelError:
		return zapcore.ErrorLevel
	case log.LevelFatal:
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Sync 刷新底层日志缓冲区。
func (l *Logger) Sync() error {
	return l.core.Sync()
}

// Close 关闭日志实例并同步缓冲区。
func (l *Logger) Close() error {
	return l.Sync()
}

// parseCaller 将 "file:line" 形式的 caller 字段转换为 zap caller。
func parseCaller(value any) zapcore.EntryCaller {
	var callerText = stripANSI(strings.TrimSpace(fmt.Sprint(value)))
	var idx = strings.LastIndexByte(callerText, ':')
	if idx <= 0 || idx >= len(callerText)-1 {
		return zapcore.EntryCaller{}
	}

	var line, err = strconv.Atoi(callerText[idx+1:])
	if err != nil {
		return zapcore.EntryCaller{}
	}

	return zapcore.EntryCaller{
		Defined: true,
		File:    filepath.ToSlash(callerText[:idx]),
		Line:    line,
	}
}

// inferCallerFromMessage 尝试从日志消息前缀中提取 "file:line" caller。
func inferCallerFromMessage(msg string) (zapcore.EntryCaller, bool) {
	if msg == "" {
		return zapcore.EntryCaller{}, false
	}

	var firstField = msg
	if idx := strings.IndexAny(msg, " \t"); idx >= 0 {
		firstField = msg[:idx]
	}
	firstField = stripANSI(strings.TrimSpace(firstField))
	if firstField == "" {
		return zapcore.EntryCaller{}, false
	}

	var caller = parseCaller(firstField)
	if !caller.Defined {
		return zapcore.EntryCaller{}, false
	}

	return caller, true
}

// shouldIgnoreField 判断是否需要忽略 provider 注入的重复标准字段。
func shouldIgnoreField(key string) bool {
	_, ok := ignoredFieldKeys[key]
	return ok
}

// isEmptyFieldValue 判断字段值是否为空，避免输出无意义的占位内容。
func isEmptyFieldValue(value any) bool {
	if value == nil {
		return true
	}

	var text = fmt.Sprint(value)
	return text == ""
}

// stripANSI 移除 ANSI 颜色控制码，避免 caller 提取被颜色前缀干扰。
func stripANSI(text string) string {
	if text == "" || !strings.Contains(text, "\x1b[") {
		return text
	}

	var builder strings.Builder
	builder.Grow(len(text))
	var inEscape = false
	var afterBracket = false
	for i := 0; i < len(text); i++ {
		c := text[i]
		if inEscape {
			if !afterBracket {
				if c == '[' {
					afterBracket = true
				}
				continue
			}
			if c >= '@' && c <= '~' {
				inEscape = false
				afterBracket = false
			}
			continue
		}
		if c == '\x1b' && i+1 < len(text) && text[i+1] == '[' {
			inEscape = true
			afterBracket = false
			continue
		}
		builder.WriteByte(c)
	}

	return builder.String()
}
