package logrus

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/logger"
)

func init() {
	_ = logger.Register(logger.Logrus, func(cfg *configv1.Logger) (*slog.Logger, error) {
		return NewLogger(cfg)
	})
}

// NewLogger 创建一个新的日志记录器 - Logrus。
func NewLogger(cfg *configv1.Logger) (*slog.Logger, error) {
	if cfg == nil || cfg.Logrus == nil {
		return nil, nil
	}

	var loggerLevel logrus.Level
	var err error
	loggerLevel, err = logrus.ParseLevel(cfg.Logrus.Level)
	if err != nil {
		loggerLevel = logrus.InfoLevel
	}

	var wrapped = &Logger{}
	if cfg.Logrus.Filepath != "" {
		err = os.MkdirAll(cfg.Logrus.Filepath, 0o755)
		if err != nil {
			return nil, err
		}
		var fileWriter = &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Logrus.Filepath, "info.log"),
			MaxSize:    int(cfg.Logrus.MaxSize),
			MaxAge:     int(cfg.Logrus.MaxAge),
			MaxBackups: int(cfg.Logrus.MaxBackups),
			Compress:   true,
		}
		wrapped.targets = append(wrapped.targets, loggerTarget{
			log: newBackendLogger(
				loggerLevel,
				cfg.Logrus.Formatter,
				cfg.Logrus.TimestampFormat,
				true,
				cfg.Logrus.DisableTimestamp,
				fileWriter,
			),
			formatCaller: logger.FormatFileCaller,
			cleanANSI:    true,
		})
	}

	if cfg.Logrus.Filepath == "" || cfg.Logrus.EnableConsole {
		wrapped.targets = append(wrapped.targets, loggerTarget{
			log: newBackendLogger(
				loggerLevel,
				cfg.Logrus.Formatter,
				cfg.Logrus.TimestampFormat,
				cfg.Logrus.DisableColors,
				cfg.Logrus.DisableTimestamp,
				os.Stdout,
			),
			formatCaller: logger.FormatConsoleCaller,
		})
	}

	return logger.NewLegacyLogger(wrapped), nil
}

// newBackendLogger 创建指定输出目标和格式的 logrus 实例。
func newBackendLogger(
	level logrus.Level,
	formatterName string,
	timestampFormat string,
	disableColors bool,
	disableTimestamp bool,
	output io.Writer,
) *logrus.Logger {
	var backend = logrus.New()
	backend.SetLevel(level)
	backend.SetOutput(output)
	backend.SetFormatter(newFormatter(formatterName, timestampFormat, disableColors, disableTimestamp))
	return backend
}

// newFormatter 根据配置创建 logrus formatter。
func newFormatter(
	formatterName string,
	timestampFormat string,
	disableColors bool,
	disableTimestamp bool,
) logrus.Formatter {
	if strings.EqualFold(formatterName, "json") {
		return &logrus.JSONFormatter{
			DisableTimestamp: disableTimestamp,
			TimestampFormat:  timestampFormat,
		}
	}
	return &textFormatter{
		timestampFormat:  timestampFormat,
		disableColors:    disableColors,
		disableTimestamp: disableTimestamp,
	}
}
