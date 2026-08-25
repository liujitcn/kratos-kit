package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v3/transport"
	configv1 "github.com/liujitcn/kratos-kit/api/gen/go/config/v1"
	basequeue "github.com/liujitcn/kratos-kit/queue"
	"github.com/liujitcn/kratos-kit/queue/data"
)

var _ transport.Server = (*Server)(nil)

// Server 将现有 queue.Queue 适配为 Kratos transport.Server。
type Server struct {
	mu sync.Mutex

	queue    basequeue.Queue
	cleanup  func()
	started  bool
	stopped  bool
	stopping bool
}

// NewServer 创建队列 transport，默认使用本地内存队列。
func NewServer(opts ...ServerOption) (*Server, error) {
	cfg := &options{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}

	instance := cfg.instance
	var cleanup func()
	var err error
	if instance == nil {
		switch cfg.backend {
		case backendMemory:
			instance, cleanup, err = basequeue.NewQueue(nil, &configv1.Data_Queue{
				Memory: &configv1.Data_Queue_Memory{PoolSize: cfg.memorySize},
			})
		case backendRedis:
			if cfg.redisConf == nil {
				return nil, errors.New("queue transport: redis config is nil")
			}
			instance, cleanup, err = basequeue.NewQueue(cfg.redisConf, cfg.queueConf)
		default:
			return nil, fmt.Errorf("queue transport: unsupported backend %d", cfg.backend)
		}
		if err != nil {
			return nil, fmt.Errorf("create queue: %w", err)
		}
	}
	if instance == nil {
		return nil, errors.New("queue transport: queue is nil")
	}
	return &Server{queue: instance, cleanup: cleanup}, nil
}

// Start 启动队列消费，并阻塞到队列停止。
func (s *Server) Start(_ context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("queue transport server already started")
	}
	if s.stopped {
		s.mu.Unlock()
		return errors.New("queue transport server already stopped")
	}
	s.started = true
	queue := s.queue
	s.mu.Unlock()

	queue.Run()

	s.mu.Lock()
	s.started = false
	s.mu.Unlock()
	return nil
}

// Stop 停止队列消费，并执行构造阶段登记的清理函数。
func (s *Server) Stop(_ context.Context) error {
	s.mu.Lock()
	if s.stopped || s.stopping {
		s.mu.Unlock()
		return nil
	}
	s.stopping = true
	queue := s.queue
	cleanup := s.cleanup
	s.mu.Unlock()

	if cleanup != nil {
		cleanup()
	} else {
		queue.Shutdown()
	}

	s.mu.Lock()
	s.started = false
	s.stopping = false
	s.stopped = true
	s.mu.Unlock()
	return nil
}

// Append 向指定流追加消息。
func (s *Server) Append(stream Stream, message data.Message) error {
	return s.queue.Append(string(stream), message)
}

// Register 注册指定流的消费处理函数。
func (s *Server) Register(stream Stream, fn data.ConsumerFunc) {
	s.queue.Register(string(stream), fn)
}

// Queue 返回底层队列实例。
func (s *Server) Queue() basequeue.Queue {
	return s.queue
}
