package fluent

import (
	"github.com/go-kratos/kratos/v2/log"

	"github.com/liujitcn/kratos-kit/api/gen/go/conf"
	"github.com/liujitcn/kratos-kit/logger"
)

func init() {
	_ = logger.Register(logger.Fluent, func(cfg *conf.Logger) (log.Logger, error) {
		return NewLogger(cfg)
	})
}

// NewLogger 创建一个新的日志记录器 - Fluent
func NewLogger(cfg *conf.Logger) (log.Logger, error) {
	if cfg == nil || cfg.Fluent == nil {
		return nil, nil
	}

	wrapped, err := NewFluentLogger(cfg.Fluent.Endpoint)
	if err != nil {
		return nil, err
	}
	return wrapped, nil
}
