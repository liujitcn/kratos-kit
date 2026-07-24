package logger

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"

	"github.com/go-kratos/kratos/v3/log"
)

// LegacyLogger 兼容 Kratos v2 风格的 key/value 日志实现。
type LegacyLogger interface {
	Log(level log.Level, keyvals ...any) error
}

type legacyHandler struct {
	logger LegacyLogger
	attrs  []slog.Attr
	group  string
}

// NewLegacyLogger 将旧版 Kratos Logger 适配为 Kratos v3 使用的 slog.Logger。
func NewLegacyLogger(logger LegacyLogger) *slog.Logger {
	return log.NewLogger(newLegacyHandler(logger), log.WithExtractor(traceAttrs))
}

// newLegacyHandler 创建旧版日志适配 handler。
func newLegacyHandler(logger LegacyLogger) slog.Handler {
	return &legacyHandler{logger: logger}
}

// Enabled 判断当前日志级别是否允许输出。
func (h *legacyHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return h != nil && h.logger != nil
}

// Handle 将 slog.Record 转换成旧版 key/value 日志格式。
func (h *legacyHandler) Handle(_ context.Context, record slog.Record) error {
	if h == nil || h.logger == nil {
		return nil
	}

	keyvals := make([]any, 0, (len(h.attrs)+record.NumAttrs())*2+4)
	keyvals = append(keyvals, slog.MessageKey, record.Message)
	if caller := recordCaller(record); caller != "" {
		keyvals = append(keyvals, "caller", caller)
	}
	for _, attr := range h.attrs {
		keyvals = appendAttr(keyvals, h.group, attr)
	}
	record.Attrs(func(attr slog.Attr) bool {
		keyvals = appendAttr(keyvals, h.group, attr)
		return true
	})

	return h.logger.Log(record.Level, keyvals...)
}

// recordCaller 将 slog 记录中的程序计数器转换为旧版 logger 使用的 caller 字段。
func recordCaller(record slog.Record) string {
	if record.PC == 0 {
		return ""
	}

	frame, _ := runtime.CallersFrames([]uintptr{record.PC}).Next()
	if frame.File == "" || frame.Line <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", frame.File, frame.Line)
}

// WithAttrs 创建带固定字段的 handler 副本。
func (h *legacyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clone := h.clone()
	clone.attrs = append(clone.attrs, attrs...)
	return clone
}

// WithGroup 创建带字段分组的 handler 副本。
func (h *legacyHandler) WithGroup(name string) slog.Handler {
	clone := h.clone()
	if clone.group == "" {
		clone.group = name
	} else {
		clone.group += "." + name
	}
	return clone
}

// clone 复制 handler，避免 WithAttrs/WithGroup 修改原实例。
func (h *legacyHandler) clone() *legacyHandler {
	if h == nil {
		return &legacyHandler{}
	}
	clone := *h
	clone.attrs = append([]slog.Attr(nil), h.attrs...)
	return &clone
}

// appendAttr 将 slog.Attr 展开为旧版 key/value 字段。
func appendAttr(keyvals []any, group string, attr slog.Attr) []any {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return keyvals
	}

	key := attr.Key
	if group != "" {
		key = group + "." + key
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, groupAttr := range attr.Value.Group() {
			keyvals = appendAttr(keyvals, key, groupAttr)
		}
		return keyvals
	}

	return append(keyvals, key, attr.Value.Any())
}
