package cron

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v3/log"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/robfig/cron/v3"
)

var (
	_ transport.Server     = (*Server)(nil)
	_ transport.Endpointer = (*Server)(nil)
)

// Server 将 Cron 调度器接入 Kratos 服务生命周期。
type Server struct {
	sync.RWMutex

	started  atomic.Bool // 服务是否已启动
	stopping atomic.Bool // 服务是否正在停止

	err error

	gracefullyShutdown bool

	cronScheduler *cron.Cron // cron 调度器
	cronLogger    cron.Logger
	cronLocation  *time.Location

	entryIDs sync.Map     // 存储所有任务ID
	cronMu   sync.RWMutex // cron 操作锁
	taskMu   sync.RWMutex // 任务执行器注册锁
	tasks    map[string]*Task
}

// NewServer 创建 Cron 服务。
func NewServer(opts ...ServerOption) *Server {
	// 初始化 cron 解析器（支持标准表达式 + 秒级）
	cronParser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)

	srv := &Server{
		started:  atomic.Bool{},
		stopping: atomic.Bool{},

		gracefullyShutdown: true,

		cronLogger: cron.DefaultLogger,
		entryIDs:   sync.Map{},
		tasks:      make(map[string]*Task),
	}

	srv.init(opts...)
	cronOptions := []cron.Option{
		cron.WithParser(cronParser),
		cron.WithLogger(srv.cronLogger),
		cron.WithChain(cron.Recover(srv.cronLogger)),
	}
	if srv.cronLocation != nil {
		cronOptions = append(cronOptions, cron.WithLocation(srv.cronLocation))
	}
	srv.cronScheduler = cron.New(cronOptions...)

	return srv
}

// init 应用 Cron 服务选项。
func (s *Server) init(opts ...ServerOption) {
	for _, o := range opts {
		o(s)
	}
}

// Name 返回 Cron 服务名称。
func (s *Server) Name() string {
	return KindCron
}

// Start 启动 Cron 服务。
func (s *Server) Start(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()

	if s.started.Load() {
		return errors.New("[Cron] server already started")
	}
	if s.err != nil {
		return s.err
	}

	log.Info("[Cron] server starting...")

	// 启动 cron 调度器
	s.cronScheduler.Start()
	s.started.Store(true)

	log.Info("[Cron] server started successfully")

	return nil
}

// Stop 停止 Cron 服务。
func (s *Server) Stop(ctx context.Context) error {
	s.Lock()
	defer s.Unlock()

	if !s.started.Load() {
		return nil
	}
	if s.stopping.Load() {
		return nil
	}

	s.stopping.Store(true)
	defer func() {
		s.stopping.Store(false)
		s.started.Store(false)
	}()

	log.Info("[Cron] server stopping...")

	// 优雅停止所有定时任务
	ctxStop, cancelStop := context.WithTimeout(ctx, 10*time.Second)
	defer cancelStop()
	jobsStopped := s.cronScheduler.Stop()
	if s.gracefullyShutdown {
		// 等待正在执行的任务结束，但不超过调用方的关闭期限。
		select {
		case <-jobsStopped.Done():
			log.Info("all cron jobs stopped gracefully")
		case <-ctxStop.Done():
			log.Warn("[Cron] server shutdown timeout, force stop")
		}
	}

	s.err = nil

	log.Info("[Cron] server stopped successfully")
	return nil
}

// Endpoint 返回 Cron 服务注册端点。
func (s *Server) Endpoint() (*url.URL, error) {
	return &url.URL{Scheme: KindCron, Host: "scheduler"}, nil
}

// RegisterTask 注册按名称调用的数据库任务执行器。
func (s *Server) RegisterTask(tasks ...*Task) error {
	s.taskMu.Lock()
	defer s.taskMu.Unlock()

	registered := make(map[string]struct{}, len(tasks))
	for _, item := range tasks {
		if item.Name == "" {
			return errors.New("[Cron] task name is empty")
		}
		if item.Exec == nil {
			return fmt.Errorf("[Cron] task executor is nil: %s", item.Name)
		}
		if _, exists := s.tasks[item.Name]; exists {
			return fmt.Errorf("[Cron] task name already registered: %s", item.Name)
		}
		if _, exists := registered[item.Name]; exists {
			return fmt.Errorf("[Cron] task name already registered: %s", item.Name)
		}
		registered[item.Name] = struct{}{}
	}
	for _, item := range tasks {
		s.tasks[item.Name] = item
	}
	return nil
}

// LookupTask 按名称查询数据库任务执行器。
func (s *Server) LookupTask(name string) (*Task, bool) {
	s.taskMu.RLock()
	task, exists := s.tasks[name]
	s.taskMu.RUnlock()
	return task, exists
}

// NewTimerJob 添加定时任务，支持在服务启动前注册。
func (s *Server) NewTimerJob(spec Spec, cmd func()) (cron.EntryID, error) {
	return s.addTimerJob(spec, cmd)
}

// StartTimerJob 添加并启动一个 cron 定时任务。
func (s *Server) StartTimerJob(spec Spec, cmd func()) (cron.EntryID, error) {
	if !s.started.Load() {
		return 0, errors.New("[Cron] server not started, please start server first")
	}
	return s.addTimerJob(spec, cmd)
}

// addTimerJob 向调度器注册定时任务并记录任务标识。
func (s *Server) addTimerJob(spec Spec, cmd func()) (cron.EntryID, error) {
	s.cronMu.Lock()
	defer s.cronMu.Unlock()

	// 添加任务
	entryID, err := s.cronScheduler.AddFunc(string(spec), cmd)
	if err != nil {
		log.Error("failed to add cron job", "error", err)
		return 0, err
	}

	// 存储任务ID
	s.entryIDs.Store(entryID, spec)
	log.Info("cron job started", "id", entryID, "spec", spec)
	return entryID, nil
}

// RemoveTimerJob 根据任务标识移除定时任务。
func (s *Server) RemoveTimerJob(entryID cron.EntryID) {
	s.cronMu.Lock()
	defer s.cronMu.Unlock()

	// 移除任务
	s.cronScheduler.Remove(entryID)
	// 删除记录
	if spec, ok := s.entryIDs.LoadAndDelete(entryID); ok {
		log.Info("cron job stopped", "id", entryID, "spec", spec)
	}
}

// StopTimerJob 根据任务标识停止单个任务。
func (s *Server) StopTimerJob(entryID cron.EntryID) {
	s.RemoveTimerJob(entryID)
}

// RemoveAllJobs 移除所有定时任务。
func (s *Server) RemoveAllJobs() {
	s.cronMu.Lock()
	defer s.cronMu.Unlock()

	count := 0
	s.entryIDs.Range(func(key, _ interface{}) bool {
		if entryID, ok := key.(cron.EntryID); ok {
			s.cronScheduler.Remove(entryID)
			count++
		}
		return true
	})

	s.entryIDs = sync.Map{}
	log.Info("all cron jobs stopped", "count", count)
}

// StopAllJobs 停止所有定时任务。
func (s *Server) StopAllJobs() {
	s.RemoveAllJobs()
}

// GetJobCount 获取当前注册的任务数量。
func (s *Server) GetJobCount() int {
	s.cronMu.RLock()
	defer s.cronMu.RUnlock()

	count := 0
	s.entryIDs.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// GetEntries 返回当前调度器中的全部任务。
func (s *Server) GetEntries() []cron.Entry {
	s.cronMu.RLock()
	defer s.cronMu.RUnlock()

	return s.cronScheduler.Entries()
}

// Scheduler 返回底层调度器，调用方直接操作时需自行保证并发安全。
func (s *Server) Scheduler() *cron.Cron {
	return s.cronScheduler
}
