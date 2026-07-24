package temporal

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kratos/kratos/v3/log"
	"go.temporal.io/sdk/worker"
)

// WorkflowWorker 管理负责轮询任务的 Temporal Worker。
type WorkflowWorker struct {
	sync.RWMutex

	client  *WorkflowClient
	worker  worker.Worker
	opts    WorkerOptions
	running bool
	closed  bool
}

// NewWorker 创建 Temporal Worker。
// 创建后不会自动启动，需要调用 Start 开始轮询任务。
func (wc *WorkflowClient) NewWorker(opts WorkerOptions) (*WorkflowWorker, error) {
	if wc.client == nil {
		return nil, fmt.Errorf("temporal client is not connected")
	}

	w := worker.New(wc.client, opts.TaskQueue, opts.Options)

	// 注册默认消息工作流，便于直接消费任务队列消息。
	w.RegisterWorkflow(BrokerMessageWorkflow)

	// 注册调用方提供的自定义工作流。
	for _, wf := range opts.Workflows {
		w.RegisterWorkflow(wf)
	}

	// 注册调用方提供的 Activity。
	for _, act := range opts.Activities {
		w.RegisterActivity(act)
	}

	return &WorkflowWorker{
		client: wc,
		worker: w,
		opts:   opts,
	}, nil
}

// Start 启动 Worker 轮询任务。
func (ww *WorkflowWorker) Start() error {
	ww.Lock()
	defer ww.Unlock()

	if ww.closed {
		return fmt.Errorf("worker is already stopped")
	}
	if ww.running {
		return fmt.Errorf("worker is already running")
	}

	err := ww.worker.Start()
	if err != nil {
		return fmt.Errorf("failed to start temporal worker for task queue %s: %w", ww.opts.TaskQueue, err)
	}

	ww.running = true

	log.Info(fmt.Sprintf("started temporal worker for task queue: %s", ww.opts.TaskQueue))

	return nil
}

// Stop 优雅停止 Worker。
func (ww *WorkflowWorker) Stop() {
	ww.Lock()
	defer ww.Unlock()

	if ww.closed {
		return
	}

	// Temporal SDK 的 Stop 不能重复调用，仅在 Start 成功后关闭底层 Worker。
	if ww.running && ww.worker != nil {
		ww.worker.Stop()
	}

	ww.running = false
	ww.closed = true

	log.Info(fmt.Sprintf("stopped temporal worker for task queue: %s", ww.opts.TaskQueue))
}

// RegisterWorkflow 注册工作流函数。
// 必须在 Start 之前调用。
func (ww *WorkflowWorker) RegisterWorkflow(fn any) {
	ww.worker.RegisterWorkflow(fn)
}

// RegisterActivity 注册 Activity 函数或结构体。
// 必须在 Start 之前调用。
func (ww *WorkflowWorker) RegisterActivity(fn any) {
	ww.worker.RegisterActivity(fn)
}

// TaskQueue 返回当前 Worker 监听的任务队列名。
func (ww *WorkflowWorker) TaskQueue() string {
	return ww.opts.TaskQueue
}

// IsRunning 返回 Worker 是否已经成功启动且尚未停止。
func (ww *WorkflowWorker) IsRunning() bool {
	ww.RLock()
	defer ww.RUnlock()
	return ww.running && !ww.closed
}

// processMessageActivity 封装默认消息处理函数。
type processMessageActivity struct {
	handler func(ctx context.Context, body []byte) error
	client  *WorkflowClient
	topic   string
}

// ProcessMessage 执行默认消息处理 Activity。
func (a *processMessageActivity) ProcessMessage(ctx context.Context, body []byte) error {
	ctx, span := a.client.startConsumerSpan(ctx, a.topic)

	err := a.handler(ctx, body)
	a.client.finishConsumerSpan(span, err)
	return err
}

// StartSimpleWorker 创建带单一消息处理函数的 Worker。
// 这是从任务队列开始处理消息的最简方式。
func (wc *WorkflowClient) StartSimpleWorker(ctx context.Context, taskQueue string, handler func(ctx context.Context, body []byte) error, opts ...func(*WorkerOptions)) (*WorkflowWorker, error) {
	workerOpts := WorkerOptions{
		TaskQueue: taskQueue,
	}
	for _, o := range opts {
		o(&workerOpts)
	}

	ww, err := wc.NewWorker(workerOpts)
	if err != nil {
		return nil, err
	}

	// 注册内置消息处理 Activity，供默认 BrokerMessageWorkflow 调用。
	act := &processMessageActivity{
		handler: handler,
		client:  wc,
		topic:   taskQueue,
	}
	ww.RegisterActivity(act.ProcessMessage)

	err = ww.Start()
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		ww.Stop()
	}()

	return ww, nil
}
