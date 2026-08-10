package cron

import (
	"time"

	"github.com/robfig/cron/v3"
)

// ServerOption 配置 Cron 服务。
type ServerOption func(o *Server)

// WithGracefullyShutdown 设置停止时是否等待正在执行的任务完成。
func WithGracefullyShutdown(enable bool) ServerOption {
	return func(s *Server) {
		s.gracefullyShutdown = enable
	}
}

// WithLocation 设置 Cron 表达式使用的时区。
func WithLocation(location *time.Location) ServerOption {
	return func(s *Server) {
		s.cronLocation = location
	}
}

// WithLogger 设置 Cron 内部日志和任务 panic 恢复日志。
func WithLogger(logger cron.Logger) ServerOption {
	return func(s *Server) {
		if logger != nil {
			s.cronLogger = logger
		}
	}
}
