package logrus

import (
	"github.com/sirupsen/logrus"

	"github.com/go-kratos/kratos/v3/log"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

type loggerTarget struct {
	log          *logrus.Logger
	formatCaller func(string) string
	cleanANSI    bool
}

// Logger 将 Kratos 日志分发到一个或多个 logrus 输出目标。
type Logger struct {
	targets []loggerTarget
}

// NewLogrusLogger 创建兼容旧版 Kratos key/value 格式的 logrus logger。
func NewLogrusLogger(logrusLogger *logrus.Logger) *Logger {
	return &Logger{
		targets: []loggerTarget{{
			log:          logrusLogger,
			formatCaller: kitlogger.FormatConsoleCaller,
		}},
	}
}

// Log 将 Kratos 日志级别和 key/value 字段写入全部 logrus 输出目标。
func (l *Logger) Log(level log.Level, keyvals ...any) error {
	if l == nil || len(l.targets) == 0 {
		return nil
	}

	var entry kitlogger.Entry
	var err error
	entry, err = kitlogger.ParseLegacyEntry(keyvals...)
	if err != nil {
		return err
	}

	var logrusLevel = toLogrusLevel(level)
	for _, target := range l.targets {
		if target.log == nil || logrusLevel > target.log.Level {
			continue
		}

		var fields = make(logrus.Fields, len(entry.Fields)+1)
		for _, field := range entry.Fields {
			fields[field.Key] = cleanFieldValue(field.Value, target.cleanANSI)
		}
		var caller = target.formatCaller(entry.Caller)
		if caller != "" {
			fields[kitlogger.CallerKey] = caller
		}

		var message = entry.Message
		if target.cleanANSI {
			message = kitlogger.CleanANSI(message)
		}
		target.log.WithFields(fields).Log(logrusLevel, message)
	}

	return nil
}

// toLogrusLevel 将 Kratos 日志级别转换为 logrus 日志级别。
func toLogrusLevel(level log.Level) logrus.Level {
	switch level {
	case log.LevelDebug:
		return logrus.DebugLevel
	case log.LevelInfo:
		return logrus.InfoLevel
	case log.LevelWarn:
		return logrus.WarnLevel
	case log.LevelError:
		return logrus.ErrorLevel
	case log.LevelFatal:
		return logrus.FatalLevel
	default:
		return logrus.InfoLevel
	}
}

// cleanFieldValue 按输出目标需要清理字符串字段中的 ANSI 控制码。
func cleanFieldValue(value any, cleanANSI bool) any {
	if !cleanANSI {
		return value
	}
	var text, ok = value.(string)
	if !ok {
		return value
	}
	return kitlogger.CleanANSI(text)
}
