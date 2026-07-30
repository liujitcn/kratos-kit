package cron

import (
	"time"

	"github.com/robfig/cron/v3"
)

// ServerOption 配置 Cron 服务。
type ServerOption func(o *Server)

// WithEnableKeepAlive 保留旧配置兼容，Cron 不再启动重复的 gRPC health 服务。
// Deprecated: Cron 使用虚拟注册端点，健康检查应由应用现有的 Kratos gRPC Server 提供。
func WithEnableKeepAlive(_ bool) ServerOption {
	return func(*Server) {}
}

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
