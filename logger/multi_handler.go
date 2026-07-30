package logger

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// NewMultiHandler 创建向多个 slog Handler 分发日志的复合 Handler。
func NewMultiHandler(handlers ...slog.Handler) slog.Handler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, handler := range handlers {
		if handler != nil {
			filtered = append(filtered, handler)
		}
	}
	return &multiHandler{handlers: filtered}
}

// NewMultiLogger 创建向多个 slog Handler 分发日志的 Logger。
func NewMultiLogger(handlers ...slog.Handler) *slog.Logger {
	return slog.New(NewMultiHandler(handlers...))
}

type multiHandler struct {
	handlers []slog.Handler
}

// Enabled 判断是否至少有一个下游 Handler 接受指定级别。
func (h *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range h.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle 将日志记录分发到全部已启用的下游 Handler。
func (h *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	handleErrors := make([]error, 0)
	for _, handler := range h.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		handleErr := handleRecord(ctx, handler, record.Clone())
		if handleErr != nil {
			handleErrors = append(handleErrors, handleErr)
		}
	}
	return errors.Join(handleErrors...)
}

// WithAttrs 为每个下游 Handler 附加相同属性。
func (h *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

// WithGroup 为每个下游 Handler 创建相同字段分组。
func (h *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(h.handlers))
	for index, handler := range h.handlers {
		handlers[index] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

// handleRecord 隔离单个下游 Handler 的异常，确保其余 Handler 仍能收到日志。
func handleRecord(ctx context.Context, handler slog.Handler, record slog.Record) (err error) {
	defer func() {
		panicValue := recover()
		if panicValue != nil {
			err = fmt.Errorf("logger handler panic: %v", panicValue)
		}
	}()
	return handler.Handle(ctx, record)
}
