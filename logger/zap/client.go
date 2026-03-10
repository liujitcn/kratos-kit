package zap

import (
	"os"
	"strconv"
	"strings"
	"time"

	zapLogger "github.com/go-kratos/kratos/contrib/log/zap/v2"
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

	//获取编码器
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	} //指定时间格式
	encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder //按级别显示不同颜色，不需要的话取值zapcore.CapitalLevelEncoder就可以了
	//encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder      //显示完整文件路径
	encoderConfig.EncodeCaller = nCallerEncoder //自定义Caller显示
	// NewJSONEncoder()输出json格式，NewConsoleEncoder()输出普通文本格式
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

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

	var core zapcore.Core
	if cfg.Zap.LogToConsole {
		core = zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writeSyncer, zapcore.AddSync(os.Stdout)), lvl)
	} else {
		core = zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writeSyncer), lvl)
	}

	var coreArr []zapcore.Core
	coreArr = append(coreArr, core)
	l := zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddCallerSkip(1)).WithOptions()

	wrapped := zapLogger.NewLogger(l)

	return wrapped, nil
}

func nCallerEncoder(caller zapcore.EntryCaller, encoder zapcore.PrimitiveArrayEncoder) {
	ss := trimPath(caller) + ":" + strconv.FormatInt(int64(caller.Line), 10)
	encoder.AppendString(ss)
}

func trimPath(caller zapcore.EntryCaller) string {
	idx0 := strings.LastIndexByte(caller.File, '/')
	if idx0 == -1 {
		return caller.File
	}
	idx1 := strings.LastIndexByte(caller.File[:idx0], '/')
	if idx1 == -1 {
		return caller.File
	}

	idx2 := strings.LastIndex(caller.File, ".git@")
	if idx2 == -1 {
		return caller.File[idx0+1:]
	}
	idx3 := strings.LastIndexByte(caller.File[:idx2], '/')
	if idx3 == -1 {
		return caller.File[idx0+1:]
	}
	var bd strings.Builder
	bd.Grow(idx2 - idx3)
	bd.WriteByte('[')
	cg := false
	for i := idx3 + 1; i < idx2; i++ {
		c := caller.File[i]
		if c == '!' {
			cg = true
		} else if cg {
			if c >= 'a' && c <= 'z' {
				c = c + 'A' - 'a'
			}
			bd.WriteByte(c)
			cg = false
		} else {
			bd.WriteByte(c)
		}
	}
	bd.WriteByte(']')
	prefix := bd.String()

	if idx3 == idx1 {
		return prefix + caller.File[idx0+1:]
	}
	return prefix + caller.File[idx1+1:]
}
