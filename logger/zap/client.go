package zap

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/logger"
)

func init() {
	_ = logger.Register(logger.Zap, func(cfg *conf.Logger) (log.Logger, error) {
		return NewLogger(cfg)
	})
}

// NewLogger 创建一个新的日志记录器 - Zap
func NewLogger(cfg *conf.Logger) (log.Logger, error) {
	if cfg == nil || cfg.Zap == nil {
		return nil, nil
	}
	if err := os.MkdirAll(cfg.Zap.Filepath, 0o755); err != nil {
		return nil, err
	}

	lumberJackLogger := &lumberjack.Logger{
		Filename:   cfg.Zap.Filepath + "/info.log",
		MaxSize:    int(cfg.Zap.MaxSize),
		MaxAge:     int(cfg.Zap.MaxAge),
		MaxBackups: int(cfg.Zap.MaxBackups),
		Compress:   true,
	}
	writeSyncer := zapcore.AddSync(lumberJackLogger)

	var lvl = new(zapcore.Level)
	if err := lvl.UnmarshalText([]byte(cfg.Zap.Level)); err != nil {
		return nil, err
	}

	var coreArr []zapcore.Core
	// 文件日志保留短路径，减少日志文件长度。
	var fileEncoder = zapcore.NewConsoleEncoder(newEncoderConfig(zapcore.CapitalLevelEncoder, nFileCallerEncoder))
	coreArr = append(coreArr, zapcore.NewCore(fileEncoder, zapcore.NewMultiWriteSyncer(writeSyncer), lvl))

	if cfg.Zap.EnableConsole {
		// 控制台输出保留颜色并打印绝对路径，方便 IDE/终端直接点击定位源码。
		var consoleEncoder = zapcore.NewConsoleEncoder(newEncoderConfig(zapcore.CapitalColorLevelEncoder, nConsoleCallerEncoder))
		coreArr = append(coreArr, zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), lvl))
	}

	return NewZapLogger(zapcore.NewTee(coreArr...)), nil
}

// newEncoderConfig 创建控制台文本编码配置。
func newEncoderConfig(levelEncoder zapcore.LevelEncoder, callerEncoder zapcore.CallerEncoder) zapcore.EncoderConfig {
	var encoderConfig = zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	} // 指定时间格式。
	encoderConfig.EncodeLevel = levelEncoder
	encoderConfig.EncodeCaller = callerEncoder
	encoderConfig.ConsoleSeparator = " " // 使用固定单空格分隔字段，保证 caller 后只有统一一个间隔。
	return encoderConfig
}

// nConsoleCallerEncoder 输出控制台 caller，保留绝对路径，便于点击跳转源码。
func nConsoleCallerEncoder(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
	ss := formatConsoleCallerPath(caller.File) + ":" + strconv.FormatInt(int64(caller.Line), 10)
	encoder.AppendString(ss)
}

// nFileCallerEncoder 输出文件 caller，统一压缩为短路径，避免日志文件过长。
func nFileCallerEncoder(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
	ss := normalizeCallerPath(caller.File) + ":" + strconv.FormatInt(int64(caller.Line), 10)
	encoder.AppendString(ss)
}

// formatConsoleCallerPath 统一控制台 caller 路径格式，优先保留绝对路径。
func formatConsoleCallerPath(file string) string {
	return filepath.ToSlash(file)
}
