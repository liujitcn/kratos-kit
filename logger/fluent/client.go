package fluent

import (
	"log/slog"

	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	"github.com/liujitcn/kratos-kit/logger"
)

func init() {
	_ = logger.Register(logger.Fluent, func(cfg *configv1.Logger) (*slog.Logger, error) {
		return NewLogger(cfg)
	})
}

// NewLogger 创建一个新的日志记录器 - Fluent
func NewLogger(cfg *configv1.Logger) (*slog.Logger, error) {
	if cfg == nil || cfg.Fluent == nil {
		return nil, nil
	}

	wrapped, err := NewFluentLogger(cfg.Fluent.Endpoint)
	if err != nil {
		return nil, err
	}
	return logger.NewLegacyLogger(wrapped), nil
}
