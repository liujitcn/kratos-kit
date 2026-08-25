package logrus

import (
	"encoding/json"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/liujitcn/kratos-kit/logger"
)

const defaultTimestampFormat = "2006-01-02 15:04:05.000"

type textFormatter struct {
	timestampFormat  string
	disableColors    bool
	disableTimestamp bool
}

// Format 将 logrus 条目格式化为与 zap 一致的控制台文本布局。
func (f *textFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var builder strings.Builder
	if !f.disableTimestamp {
		var timestampFormat = f.timestampFormat
		if timestampFormat == "" {
			timestampFormat = defaultTimestampFormat
		}
		builder.WriteString(entry.Time.Format(timestampFormat))
		builder.WriteByte(' ')
	}

	var level = formatLevel(entry.Level)
	if !f.disableColors {
		builder.WriteString(levelColor(entry.Level))
		builder.WriteString(level)
		builder.WriteString("\x1b[0m")
	} else {
		builder.WriteString(level)
	}

	var caller = entry.Data[logger.CallerKey]
	if caller != nil && caller != "" {
		builder.WriteByte(' ')
		builder.WriteString(entryString(caller))
	}
	if entry.Message != "" {
		builder.WriteByte(' ')
		builder.WriteString(entry.Message)
	}

	var fields = make(map[string]any, len(entry.Data))
	for key, value := range entry.Data {
		if key == logger.CallerKey {
			continue
		}
		fields[key] = value
	}
	if len(fields) > 0 {
		var encodedFields, err = logger.FormatFields(fields)
		if err != nil {
			return nil, err
		}
		builder.WriteByte(' ')
		builder.WriteString(encodedFields)
	}
	builder.WriteByte('\n')
	return []byte(builder.String()), nil
}

// formatLevel 返回与 zap 一致的全大写日志级别。
func formatLevel(level logrus.Level) string {
	switch level {
	case logrus.TraceLevel:
		return "TRACE"
	case logrus.DebugLevel:
		return "DEBUG"
	case logrus.InfoLevel:
		return "INFO"
	case logrus.WarnLevel:
		return "WARN"
	case logrus.ErrorLevel:
		return "ERROR"
	case logrus.FatalLevel:
		return "FATAL"
	case logrus.PanicLevel:
		return "PANIC"
	default:
		return strings.ToUpper(level.String())
	}
}

// levelColor 返回与 zap 控制台编码器相近的级别颜色。
func levelColor(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "\x1b[35m"
	case logrus.WarnLevel:
		return "\x1b[33m"
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		return "\x1b[31m"
	default:
		return "\x1b[34m"
	}
}

// entryString 将日志字段安全转换为字符串。
func entryString(value any) string {
	var text, ok = value.(string)
	if ok {
		return text
	}
	var encoded, err = json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}
