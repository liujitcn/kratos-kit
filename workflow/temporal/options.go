package temporal

import (
	"time"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"

	enumspb "go.temporal.io/api/enums/v1"
)

////////////////////////////////////////////////////////////////////////////////
/// 客户端配置
////////////////////////////////////////////////////////////////////////////////

type ClientOptions struct {
	// HostPort 是 Temporal Server 地址，默认值为 "localhost:7233"。
	HostPort string

	// Namespace 是 Temporal 命名空间，默认值为 "default"。
	Namespace string
}

// WithClientHostPort 设置 Temporal Server 地址。
func WithClientHostPort(hostPort string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.HostPort = hostPort
	}
}

// WithClientNamespace 设置 Temporal 命名空间。
func WithClientNamespace(namespace string) func(*ClientOptions) {
	return func(o *ClientOptions) {
		o.Namespace = namespace
	}
}

////////////////////////////////////////////////////////////////////////////////
/// 执行工作流配置
////////////////////////////////////////////////////////////////////////////////

type ExecuteOptions struct {
	// TaskQueue 是工作流使用的任务队列。
	TaskQueue string

	// WorkflowID 是工作流执行的唯一标识。
	WorkflowID string

	// WorkflowFn 是需要执行的工作流函数，空值时使用默认工作流。
	WorkflowFn any

	// RunTimeout 是单次工作流运行的最大耗时。
	RunTimeout time.Duration

	// ExecutionTimeout 是包含重试和 continue-as-new 在内的总执行超时。
	ExecutionTimeout time.Duration

	// TaskTimeout 是单个工作流任务超时。
	TaskTimeout time.Duration

	// RetryPolicy 是工作流重试策略。
	RetryPolicy *temporal.RetryPolicy

	// CronSchedule 是工作流定时调度表达式。
	CronSchedule string

	// IDReusePolicy 控制 WorkflowID 已存在时的复用行为。
	IDReusePolicy enumspb.WorkflowIdReusePolicy
}

////////////////////////////////////////////////////////////////////////////////
/// Worker 配置
////////////////////////////////////////////////////////////////////////////////

type WorkerOptions struct {
	// TaskQueue 是 Worker 监听的任务队列。
	TaskQueue string

	// Options 是原生 Temporal Worker 配置。
	Options worker.Options

	// Workflows 是需要额外注册的工作流函数列表。
	Workflows []any

	// Activities 是需要额外注册的 Activity 函数或结构体列表。
	Activities []any
}

////////////////////////////////////////////////////////////////////////////////
/// StartWorkflowOptions 转换辅助方法
////////////////////////////////////////////////////////////////////////////////

func toStartWorkflowOptions(opts ExecuteOptions) client.StartWorkflowOptions {
	swo := client.StartWorkflowOptions{
		TaskQueue: opts.TaskQueue,
	}

	if opts.WorkflowID != "" {
		swo.ID = opts.WorkflowID
	}
	if opts.RunTimeout > 0 {
		swo.WorkflowRunTimeout = opts.RunTimeout
	}
	if opts.ExecutionTimeout > 0 {
		swo.WorkflowExecutionTimeout = opts.ExecutionTimeout
	}
	if opts.TaskTimeout > 0 {
		swo.WorkflowTaskTimeout = opts.TaskTimeout
	}
	if opts.RetryPolicy != nil {
		swo.RetryPolicy = opts.RetryPolicy
	}
	if opts.CronSchedule != "" {
		swo.CronSchedule = opts.CronSchedule
	}
	swo.WorkflowIDReusePolicy = opts.IDReusePolicy

	return swo
}
