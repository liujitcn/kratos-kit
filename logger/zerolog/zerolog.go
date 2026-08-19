package zerolog

import (
	"github.com/rs/zerolog"

	"github.com/go-kratos/kratos/v3/log"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

type loggerTarget struct {
	log          *zerolog.Logger
	formatCaller func(string) string
	cleanANSI    bool
}

// Logger 将 Kratos 日志分发到一个或多个 zerolog 输出目标。
type Logger struct {
	targets []loggerTarget
}

// NewZerologLogger 创建兼容旧版 Kratos key/value 格式的 zerolog logger。
func NewZerologLogger(zerologLogger *zerolog.Logger) *Logger {
	return &Logger{
		targets: []loggerTarget{{
			log:          zerologLogger,
			formatCaller: kitlogger.FormatConsoleCaller,
		}},
	}
}

// Log 将 Kratos 日志级别和 key/value 字段写入全部 zerolog 输出目标。
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

	for _, target := range l.targets {
		if target.log == nil {
			continue
		}
		var event = newEvent(target.log, level)
		for _, field := range entry.Fields {
			event = event.Any(field.Key, cleanFieldValue(field.Value, target.cleanANSI))
		}
		var caller = target.formatCaller(entry.Caller)
		if caller != "" {
			event = event.Str(zerolog.CallerFieldName, caller)
		}
		var message = entry.Message
		if target.cleanANSI {
			message = kitlogger.CleanANSI(message)
		}
		event.Msg(message)
	}
	return nil
}

// newEvent 根据 Kratos 级别创建 zerolog 事件。
func newEvent(zerologLogger *zerolog.Logger, level log.Level) *zerolog.Event {
	switch level {
	case log.LevelDebug:
		return zerologLogger.Debug()
	case log.LevelInfo:
		return zerologLogger.Info()
	case log.LevelWarn:
		return zerologLogger.Warn()
	case log.LevelError:
		return zerologLogger.Error()
	case log.LevelFatal:
		return zerologLogger.Fatal()
	default:
		return zerologLogger.Info()
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
