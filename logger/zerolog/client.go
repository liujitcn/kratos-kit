package zerolog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	kitlogger "github.com/liujitcn/kratos-kit/logger"
)

const defaultTimeFieldFormat = "2006-01-02 15:04:05.000"

func init() {
	_ = kitlogger.Register(kitlogger.Zerolog, func(cfg *configv1.Logger) (*slog.Logger, error) {
		return NewLogger(cfg)
	})
}

// NewLogger 创建一个新的日志记录器 - Zerolog。
func NewLogger(cfg *configv1.Logger) (*slog.Logger, error) {
	if cfg == nil || cfg.Zerolog == nil {
		return nil, nil
	}

	configureGlobals(cfg.Zerolog)
	var level zerolog.Level
	var err error
	level, err = zerolog.ParseLevel(strings.ToLower(cfg.Zerolog.Level))
	if err != nil {
		level = zerolog.InfoLevel
	}

	var wrapped = &Logger{}
	var writerKind = strings.ToLower(cfg.Zerolog.Writer)
	if writerKind == "" {
		writerKind = "stdout"
	}
	switch writerKind {
	case "stdout", "console":
		wrapped.targets = append(wrapped.targets, newLoggerTarget(os.Stdout, level, kitlogger.FormatConsoleCaller, false, true))
	case "stderr":
		wrapped.targets = append(wrapped.targets, newLoggerTarget(os.Stderr, level, kitlogger.FormatConsoleCaller, false, true))
	case "file", "lumberjack":
		if cfg.Zerolog.Filepath == "" {
			return nil, fmt.Errorf("zerolog filepath is required for %s writer", writerKind)
		}
		err = os.MkdirAll(cfg.Zerolog.Filepath, 0o755)
		if err != nil {
			return nil, err
		}
		var fileWriter = NewLumberjackWriter(
			filepath.Join(cfg.Zerolog.Filepath, "info.log"),
			int(cfg.Zerolog.MaxSize),
			int(cfg.Zerolog.MaxBackups),
			int(cfg.Zerolog.MaxAge),
			true,
		)
		wrapped.targets = append(wrapped.targets, newLoggerTarget(fileWriter, level, kitlogger.FormatFileCaller, true, false))
		if cfg.Zerolog.EnableConsole {
			wrapped.targets = append(wrapped.targets, newLoggerTarget(os.Stdout, level, kitlogger.FormatConsoleCaller, false, true))
		}
	default:
		return nil, fmt.Errorf("unsupported zerolog writer: %s", writerKind)
	}

	return kitlogger.NewLegacyLogger(wrapped), nil
}

// configureGlobals 应用 zerolog 的全局字段名和时间格式配置。
func configureGlobals(cfg *configv1.Logger_Zerolog) {
	zerolog.TimeFieldFormat = defaultTimeFieldFormat
	zerolog.TimestampFieldName = "time"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "message"
	zerolog.CallerFieldName = "caller"
	if cfg.TimeFieldFormat != "" {
		zerolog.TimeFieldFormat = cfg.TimeFieldFormat
	}
	if cfg.TimestampFieldName != "" {
		zerolog.TimestampFieldName = cfg.TimestampFieldName
	}
	if cfg.LevelFieldName != "" {
		zerolog.LevelFieldName = cfg.LevelFieldName
	}
	if cfg.MessageFieldName != "" {
		zerolog.MessageFieldName = cfg.MessageFieldName
	}
}

// newLoggerTarget 创建一个具有独立 caller 格式的 zerolog 输出目标。
func newLoggerTarget(
	output io.Writer,
	level zerolog.Level,
	formatCaller func(string) string,
	cleanANSI bool,
	colorLevel bool,
) loggerTarget {
	var writer = newTextWriter(
		output,
		zerolog.TimestampFieldName,
		zerolog.LevelFieldName,
		zerolog.MessageFieldName,
		zerolog.CallerFieldName,
		colorLevel,
	)
	var backend = zerolog.New(writer).Level(level).With().Timestamp().Logger()
	return loggerTarget{
		log:          &backend,
		formatCaller: formatCaller,
		cleanANSI:    cleanANSI,
	}
}
