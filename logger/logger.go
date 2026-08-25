package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/go-kratos/kratos/v3/log"
	"go.opentelemetry.io/otel/trace"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
)

// NewLogger 动态创建日志实例
func NewLogger(cfg *configv1.Logger) (*slog.Logger, error) {
	if cfg == nil {
		return nil, nil
	}

	if cfg.GetType() == "" || cfg.GetType() == string(Std) {
		return NewStdLogger(), nil
	}

	// normalize to lower case for lookup
	typ := Type(strings.ToLower(cfg.GetType()))
	norm := Type(strings.ToLower(string(typ)))

	f, ok := GetFactory(norm)
	if !ok {
		// prepare available list for helpful error
		available := ListFactories()
		strs := make([]string, 0, len(available))
		for _, t := range available {
			strs = append(strs, string(t))
		}
		slices.Sort(strs)
		return nil, fmt.Errorf("unsupported logger type: %s; available: %v", typ, strs)
	}

	lg, err := f(cfg)
	if err != nil {
		return nil, fmt.Errorf("create logger %s: %w", typ, err)
	}
	if lg == nil {
		return nil, fmt.Errorf("logger factory %s returned nil logger", typ)
	}
	return lg, nil
}

// NewLoggerProvider 创建一个新的日志记录器提供者
// 它会从 cfg 创建具体 logger，并为 logger 附加 service.* 标准字段。
func NewLoggerProvider(cfg *configv1.Logger, appInfo *configv1.AppInfo) *slog.Logger {
	var l *slog.Logger
	if cfg == nil || cfg.GetType() == "" {
		l = NewStdLogger()
	} else {
		// 创建指定类型 logger 失败时回退到标准控制台 logger，避免启动期日志不可用。
		if lg, err := NewLogger(cfg); err == nil && lg != nil {
			l = lg
		} else {
			l = NewStdLogger()
		}
	}

	fields := make([]any, 0, 3)
	if appInfo != nil {
		fields = append(fields,
			"service.id", appInfo.GetAppId(),
			"service.instance", appInfo.GetInstanceId(),
			"service.version", appInfo.GetVersion(),
		)
	}

	if len(fields) == 0 {
		return l
	}
	return l.With(fields...)
}

// NewStdLogger 创建一个新的日志记录器 - Kratos内置，控制台输出
func NewStdLogger() *slog.Logger {
	handler := log.NewHandler(
		log.WithWriter(os.Stdout),
		log.WithFormat(log.FormatText),
		log.WithAddSource(true),
		log.WithExtractor(traceAttrs),
	)
	return log.NewLogger(handler)
}

func traceAttrs(ctx context.Context) []slog.Attr {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil
	}

	attrs := make([]slog.Attr, 0, 2)
	if spanCtx.HasTraceID() {
		attrs = append(attrs, slog.String("trace_id", spanCtx.TraceID().String()))
	}
	if spanCtx.HasSpanID() {
		attrs = append(attrs, slog.String("span_id", spanCtx.SpanID().String()))
	}
	return attrs
}
