package conductor

import (
	"fmt"
	"sync"
	"time"

	"github.com/conductor-sdk/conductor-go/sdk/model"
	"github.com/conductor-sdk/conductor-go/sdk/worker"
	"github.com/go-kratos/kratos/v3/log"
)

// TaskHandler 是处理 Conductor Task 的函数签名。
// 入参是任务内容，返回任务输出或错误。
type TaskHandler = model.ExecuteTaskFunction

// TaskWorker 管理负责轮询并执行 Conductor Task 的 Worker。
type TaskWorker struct {
	mu         sync.RWMutex
	client     *WorkflowClient
	taskRunner *worker.TaskRunner
	config     WorkerConfig
	stopped    bool
}

// StartWorker 在当前客户端上启动指定任务类型的 Worker。
// 这是开始处理 Conductor Task 的最简方式。
func (wc *WorkflowClient) StartWorker(taskType string, handler TaskHandler, concurrency int, pollInterval time.Duration) (*TaskWorker, error) {
	if wc.apiClient == nil {
		return nil, fmt.Errorf("api client is nil")
	}

	// 未显式指定并发数时，使用模块默认值，避免传入无效批量大小。
	if concurrency <= 0 {
		concurrency = defaultConcurrency
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	taskRunner := worker.NewTaskRunnerWithApiClient(wc.apiClient)

	tw := &TaskWorker{
		client:     wc,
		taskRunner: taskRunner,
		config: WorkerConfig{
			TaskType:     taskType,
			Concurrency:  concurrency,
			PollInterval: pollInterval,
		},
	}

	err := taskRunner.StartWorker(taskType, handler, concurrency, pollInterval)
	if err != nil {
		return nil, fmt.Errorf("start worker error: %w", err)
	}

	log.Info(fmt.Sprintf("started worker for task type: %s (concurrency: %d)", taskType, concurrency))

	return tw, nil
}

// StartWorkerWithConfig 使用完整配置启动任务 Worker。
func (wc *WorkflowClient) StartWorkerWithConfig(config WorkerConfig, handler TaskHandler) (*TaskWorker, error) {
	if wc.apiClient == nil {
		return nil, fmt.Errorf("api client is nil")
	}

	// 未显式指定并发数时，使用模块默认值，避免传入无效批量大小。
	if config.Concurrency <= 0 {
		config.Concurrency = defaultConcurrency
	}
	if config.PollInterval <= 0 {
		config.PollInterval = defaultPollInterval
	}

	taskRunner := worker.NewTaskRunnerWithApiClient(wc.apiClient)

	tw := &TaskWorker{
		client:     wc,
		taskRunner: taskRunner,
		config:     config,
	}

	err := taskRunner.StartWorkerWithDomain(config.TaskType, handler, config.Concurrency, config.PollInterval, config.Domain)
	if err != nil {
		return nil, fmt.Errorf("start worker error: %w", err)
	}

	log.Info(fmt.Sprintf("started worker for task type: %s (concurrency: %d, poll: %s)",
		config.TaskType, config.Concurrency, config.PollInterval))

	return tw, nil
}

// Stop 停止当前任务类型的底层轮询。
func (tw *TaskWorker) Stop() {
	tw.mu.Lock()
	defer tw.mu.Unlock()

	if tw.stopped {
		return
	}

	// Shutdown 会删除该任务类型的轮询配置，底层 Worker 会在当前轮询循环结束后退出。
	tw.taskRunner.Shutdown(tw.config.TaskType)
	tw.stopped = true
	log.Info(fmt.Sprintf("stopped worker for task type %s", tw.config.TaskType))
}

// WaitForCompletion 等待底层 Worker 完成退出。
func (tw *TaskWorker) WaitForCompletion() {
	tw.taskRunner.WaitWorkers()
}

// TaskType 返回当前 Worker 轮询的任务类型。
func (tw *TaskWorker) TaskType() string {
	return tw.config.TaskType
}

// IsRunning 返回当前 Worker 是否仍处于运行状态。
func (tw *TaskWorker) IsRunning() bool {
	tw.mu.RLock()
	defer tw.mu.RUnlock()
	return !tw.stopped
}
