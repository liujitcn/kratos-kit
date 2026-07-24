package goworkflows

import (
	"context"
	"fmt"
	"sync"

	"github.com/cschleiden/go-workflows/backend"
	"github.com/cschleiden/go-workflows/worker"
	"github.com/cschleiden/go-workflows/workflow"
	"github.com/go-kratos/kratos/v3/log"
)

// WorkflowWorker 使用 go-workflows worker 管理工作流和 Activity 执行。
type WorkflowWorker struct {
	mu      sync.RWMutex
	backend backend.Backend
	worker  *worker.Worker
	opts    *WorkerOptions
	ctx     context.Context
	cancel  context.CancelFunc
	running bool
}

// NewWorker 创建同时处理工作流和 Activity 的 Worker。
func NewWorker(b backend.Backend, opts *WorkerOptions) (*WorkflowWorker, error) {
	if b == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	workerOpts := opts.toWorkerOptions()
	w := worker.New(b, &workerOpts)

	return &WorkflowWorker{
		backend: b,
		worker:  w,
		opts:    opts,
	}, nil
}

// NewWorkflowOnlyWorker 创建仅处理工作流任务的 Worker。
func NewWorkflowOnlyWorker(b backend.Backend, opts *WorkerOptions) (*WorkflowWorker, error) {
	if b == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	var workflowOpts *worker.WorkflowWorkerOptions
	if opts != nil {
		workerOpts := opts.toWorkerOptions()
		workflowOpts = &workerOpts.WorkflowWorkerOptions
	}

	w := worker.NewWorkflowWorker(b, workflowOpts)

	return &WorkflowWorker{
		backend: b,
		worker:  w,
		opts:    opts,
	}, nil
}

// NewActivityOnlyWorker 创建仅处理 Activity 任务的 Worker。
func NewActivityOnlyWorker(b backend.Backend, opts *WorkerOptions) (*WorkflowWorker, error) {
	if b == nil {
		return nil, fmt.Errorf("backend is nil")
	}

	var activityOpts *worker.ActivityWorkerOptions
	if opts != nil {
		workerOpts := opts.toWorkerOptions()
		activityOpts = &workerOpts.ActivityWorkerOptions
	}

	w := worker.NewActivityWorker(b, activityOpts)

	return &WorkflowWorker{
		backend: b,
		worker:  w,
		opts:    opts,
	}, nil
}

// RegisterWorkflow 向 Worker 注册工作流函数。
func (ww *WorkflowWorker) RegisterWorkflow(wf workflow.Workflow) error {
	err := ww.worker.RegisterWorkflow(wf)
	if err != nil {
		return fmt.Errorf("register workflow error: %w", err)
	}
	return nil
}

// RegisterActivity 向 Worker 注册 Activity 函数或结构体。
// 结构体注册时，公开方法会作为 Activity 暴露。
func (ww *WorkflowWorker) RegisterActivity(a workflow.Activity) error {
	err := ww.worker.RegisterActivity(a)
	if err != nil {
		return fmt.Errorf("register activity error: %w", err)
	}
	return nil
}

// Start 启动 Worker，取消传入的 context 会停止后续轮询。
func (ww *WorkflowWorker) Start(ctx context.Context) error {
	ww.mu.Lock()
	if ww.running {
		ww.mu.Unlock()
		return fmt.Errorf("worker is already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	ww.ctx = ctx
	ww.cancel = cancel
	ww.running = true
	ww.mu.Unlock()

	log.Info("starting worker")

	err := ww.worker.Start(ctx)
	if err != nil {
		ww.mu.Lock()
		ww.running = false
		ww.cancel()
		ww.mu.Unlock()
		return fmt.Errorf("start worker error: %w", err)
	}

	go func() {
		<-ctx.Done()
		ww.mu.Lock()
		defer ww.mu.Unlock()

		// 仅当前运行上下文结束时更新状态，避免旧 context 影响后续重新启动。
		if ww.ctx == ctx {
			ww.running = false
		}
	}()

	return nil
}

// Stop 通过取消内部 context 通知 Worker 停止轮询。
func (ww *WorkflowWorker) Stop() {
	ww.mu.Lock()
	defer ww.mu.Unlock()

	if ww.cancel != nil {
		ww.cancel()
	}
	ww.running = false

	log.Info("worker stopped")
}

// WaitForCompletion 等待所有进行中的任务完成。
// 通常在 Stop 后调用，用于优雅排空正在执行的任务。
func (ww *WorkflowWorker) WaitForCompletion() error {
	return ww.worker.WaitForCompletion()
}

// IsRunning 返回 Worker 当前是否处于运行状态。
func (ww *WorkflowWorker) IsRunning() bool {
	ww.mu.RLock()
	defer ww.mu.RUnlock()
	return ww.running
}
